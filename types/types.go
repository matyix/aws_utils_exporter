package types

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type PrometheusSink struct {
	mu        sync.Mutex
	gauges    map[string]prometheus.Gauge
	summaries map[string]prometheus.Summary
	counters  map[string]prometheus.Counter
}

func NewPrometheusSink() (*PrometheusSink, error) {
	return &PrometheusSink{
		gauges:    make(map[string]prometheus.Gauge),
		summaries: make(map[string]prometheus.Summary),
		counters:  make(map[string]prometheus.Counter),
	}, nil
}

var (
	Region   = flag.String("region", "eu-west-1", "the region to query")
	Taglist  = flag.String("instance-tags", "", "comma seperated list of tag keys to use as metric labels")
	Duration = flag.Duration("duration", time.Minute*4, "How often to query the API")
	Address  = flag.String("addr", ":9190", "port to listen on")

	InstancesLabels = []string{
		"groups",
		"owner_id",
		"requester_id",
		"az",
		"instance_type",
		"lifecycle",
	}

	SpotLabels = []string{
		"az",
		"product",
		"persistence",
		"instance_type",
		"launch_group",
		"instance_profile",
	}

	SpotHourlyLabels = []string{
		"az",
		"product",
		"instance_type",
	}

	SpotHourlyPrice = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ec2_spot_price_per_hour_dollars",
		Help: "Current market price of a spot instance, per hour,  in dollars",
	},
		SpotHourlyLabels)
)

var InstancesCount *prometheus.GaugeVec
var InstanceTags = map[string]string{}

var SpotInstanceCount *prometheus.GaugeVec
var SpotInstanceBidPrice *prometheus.GaugeVec
var SpotInstanceBlockHourlyPrice *prometheus.GaugeVec

var InstanceLabelsCacheMutex = sync.RWMutex{}
var InstanceLabelsCache = map[string]prometheus.Labels{}
var InstanceLabelsCacheIsVPC = map[string]bool{}

const (
	TimeFormat = "2006-01-02T15:04:05Z"
)

type TerminationCollector struct {
	scrapeSuccessful     *prometheus.Desc
	terminationIndicator *prometheus.Desc
	terminationTime      *prometheus.Desc
}
