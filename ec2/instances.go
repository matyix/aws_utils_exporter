package ec2

import (
	"fmt"
	"log"

	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	types "github.com/matyix/aws_utils_exporter/types"
	"github.com/prometheus/client_golang/prometheus"
)

func Instances(svc *ec2.EC2, awsRegion string) {
	types.InstanceLabelsCacheMutex.Lock()
	defer types.InstanceLabelsCacheMutex.Unlock()

	//Clear the cache
	types.InstanceLabelsCache = map[string]prometheus.Labels{}
	types.InstanceLabelsCacheIsVPC = map[string]bool{}

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-code"),
				Values: []*string{aws.String("16")},
			},
		},
	}
	resp, err := svc.DescribeInstances(params)
	if err != nil {
		fmt.Println("there was an error listing instances in", awsRegion, err.Error())
		log.Fatal(err.Error())
	}

	types.InstancesCount.Reset()
	labels := prometheus.Labels{}
	for _, r := range resp.Reservations {
		groups := []string{}
		for _, g := range r.Groups {
			groups = append(groups, *g.GroupName)
		}
		sort.Strings(groups)
		labels["groups"] = strings.Join(groups, ",")
		labels["owner_id"] = *r.OwnerId
		labels["requester_id"] = *r.OwnerId
		if r.RequesterId != nil {
			labels["requester_id"] = *r.RequesterId
		}
		for _, ins := range r.Instances {
			labels["az"] = *ins.Placement.AvailabilityZone
			labels["instance_type"] = *ins.InstanceType
			labels["lifecycle"] = "normal"
			if ins.InstanceLifecycle != nil {
				labels["lifecycle"] = *ins.InstanceLifecycle
			}
			types.InstanceLabelsCache[*ins.InstanceId] = prometheus.Labels{}
			for _, label := range types.InstanceTags {
				labels[label] = ""
				types.InstanceLabelsCache[*ins.InstanceId][label] = ""
			}
			for _, tag := range ins.Tags {
				label, ok := types.InstanceTags[*tag.Key]
				if ok {
					labels[label] = *tag.Value
					types.InstanceLabelsCache[*ins.InstanceId][label] = *tag.Value
				}
			}
			if ins.VpcId != nil {
				types.InstanceLabelsCacheIsVPC[*ins.InstanceId] = true
			}
			types.InstancesCount.With(labels).Inc()
		}
	}
}

func SpotInstances(svc *ec2.EC2, awsRegion string) {
	types.InstanceLabelsCacheMutex.RLock()
	defer types.InstanceLabelsCacheMutex.RUnlock()

	params := &ec2.DescribeSpotInstanceRequestsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("state"),
				Values: []*string{aws.String("active")},
			},
		},
	}
	resp, err := svc.DescribeSpotInstanceRequests(params)
	if err != nil {
		fmt.Println("there was an error listing spot requests", awsRegion, err.Error())
		log.Fatal(err.Error())
	}

	productSeen := map[string]bool{}

	labels := prometheus.Labels{}
	types.SpotInstanceCount.Reset()
	types.SpotInstanceBlockHourlyPrice.Reset()
	types.SpotInstanceBidPrice.Reset()
	for _, r := range resp.SpotInstanceRequests {
		for _, label := range types.InstanceTags {
			labels[label] = ""
		}
		if r.InstanceId != nil {
			if ilabels, ok := types.InstanceLabelsCache[*r.InstanceId]; ok {
				for k, v := range ilabels {
					labels[k] = v
				}
			}
		}

		labels["az"] = *r.LaunchedAvailabilityZone

		product := *r.ProductDescription
		if isVpc, ok := types.InstanceLabelsCacheIsVPC[*r.InstanceId]; ok && isVpc {
			product += " (Amazon VPC)"
		}
		labels["product"] = product
		productSeen[product] = true

		labels["persistence"] = "one-time"
		if r.Type != nil {
			labels["persistence"] = *r.Type
		}

		labels["launch_group"] = "none"
		if r.LaunchGroup != nil {
			labels["launch_group"] = *r.LaunchGroup
		}

		labels["instance_type"] = "unknown"
		if r.LaunchSpecification != nil && r.LaunchSpecification.InstanceType != nil {
			labels["instance_type"] = *r.LaunchSpecification.InstanceType
		}

		labels["instance_profile"] = "unknown"
		if r.LaunchSpecification != nil && r.LaunchSpecification.IamInstanceProfile != nil {
			labels["instance_profile"] = *r.LaunchSpecification.IamInstanceProfile.Name
		}

		price := 0.0
		if r.ActualBlockHourlyPrice != nil {
			if f, err := strconv.ParseFloat(*r.ActualBlockHourlyPrice, 64); err == nil {
				price = f
			}
		}
		types.SpotInstanceBlockHourlyPrice.With(labels).Add(price)

		price = 0
		if r.SpotPrice != nil {
			if f, err := strconv.ParseFloat(*r.SpotPrice, 64); err == nil {
				price = f
			}
		}
		types.SpotInstanceBidPrice.With(labels).Add(price)

		types.SpotInstanceCount.With(labels).Inc()
	}

	// This is silly, but spot instances requests don't seem to include the vpc case
	pList := []*string{}
	for p := range productSeen {
		pp := p
		pList = append(pList, &pp)
	}

	phParams := &ec2.DescribeSpotPriceHistoryInput{
		StartTime: aws.Time(time.Now()),
		EndTime:   aws.Time(time.Now()),
		//		ProductDescriptions: pList,
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("product-description"),
				Values: pList,
			},
		},
	}
	phResp, err := svc.DescribeSpotPriceHistory(phParams)
	if err != nil {
		fmt.Println("there was an error listing spot requests", awsRegion, err.Error())
		log.Fatal(err.Error())
	}
	spLabels := prometheus.Labels{}
	for _, sp := range phResp.SpotPriceHistory {
		spLabels["az"] = *sp.AvailabilityZone
		spLabels["product"] = *sp.ProductDescription
		spLabels["instance_type"] = *sp.InstanceType
		if sp.SpotPrice != nil {
			if f, err := strconv.ParseFloat(*sp.SpotPrice, 64); err == nil {
				types.SpotHourlyPrice.With(spLabels).Set(f)
			}
		}
	}
}

var cleanre = regexp.MustCompile("[^A-Za-z0-9]")

func Tagname(n string) string {
	c := cleanre.ReplaceAllString(n, "_")
	c = strings.ToLower(strings.Trim(c, "_"))
	return "aws_tag_" + c
}
