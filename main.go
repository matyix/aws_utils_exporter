package main

import (
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2service "github.com/aws/aws-sdk-go/service/ec2"
	ec2 "github.com/matyix/aws_utils_exporter/ec2"
	types "github.com/matyix/aws_utils_exporter/types"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	flag.Parse()

	tagl := []string{}
	for _, tstr := range strings.Split(*types.Taglist, ",") {
		ctag := ec2.Tagname(tstr)
		types.InstanceTags[tstr] = ctag
		tagl = append(tagl, ctag)
	}
	types.InstancesCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ec2_instances_count",
		Help: "End time of this reservation",
	},
		append(types.InstancesLabels, tagl...))

	types.SpotInstanceCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ec2_spot_request_count",
		Help: "Number of active/fullfilled spot requests",
	},
		append(types.SpotLabels, tagl...))
	types.SpotInstanceBidPrice = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ec2_spot_request_bid_price_hourly_dollars",
		Help: "cost of spot instances hourly usage in dollars",
	},
		append(types.SpotLabels, tagl...))
	types.SpotInstanceBlockHourlyPrice = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ec2_spot_request_actual_block_price_hourly_dollars",
		Help: "fixed hourly cost of limited duration spot instances in dollars",
	},
		append(types.SpotLabels, tagl...))

	prometheus.Register(types.InstancesCount)
	prometheus.Register(types.SpotInstanceCount)
	prometheus.Register(types.SpotInstanceBidPrice)
	prometheus.Register(types.SpotInstanceBlockHourlyPrice)
	prometheus.Register(types.SpotHourlyPrice)

	sess, err := session.NewSession()
	if err != nil {
		log.Fatalf("failed to create session %v\n", err)
	}

	svc := ec2service.New(sess, &aws.Config{Region: aws.String(*types.Region)})

	go func() {
		for {
			ec2.Instances(svc, *types.Region)
			go ec2.SpotInstances(svc, *types.Region)
			<-time.After(*types.Duration)
		}
	}()

	http.Handle("/metrics", prometheus.Handler())

	log.Println(http.ListenAndServe(*types.Address, nil))
}
