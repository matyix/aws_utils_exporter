package ec2

import (
	"time"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/matyix/aws_utils_exporter/types"
	"github.com/prometheus/client_golang/prometheus"
)

/**
func NewTerminationCollector() *types.TerminationCollector {
	return &types.TerminationCollector{
		scrapeSuccessful:     prometheus.NewDesc("metadata_service_available", "Metadata service available", nil, nil),
		terminationIndicator: prometheus.NewDesc("termination_imminent", "Instance is about to be terminated", nil, nil),
		terminationTime:      prometheus.NewDesc("termination_in", "Instance will be terminated in", nil, nil),
	}
}

func (c *types.TerminationCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.scrapeSuccessful
	ch <- c.terminationIndicator
	ch <- c.terminationTime

}
**/

func (c *types.TerminationCollector) Collect(ch chan<- prometheus.Metric) {
	log.Info("Fetching termination data from metadata-service")
	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/spot/termination-time")
	if err != nil {
		log.Errorf("Failed to fetch data from metadata service: %s", err)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccessful, prometheus.GaugeValue, 0)
		return
	} else {
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccessful, prometheus.GaugeValue, 1)

		if resp.StatusCode == 404 {
			ch <- prometheus.MustNewConstMetric(c.terminationIndicator, prometheus.GaugeValue, 0)
			return
		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)

			// value may be present but not be a time according to AWS docs,
			// so parse error is not fatal
			termtime, err := time.Parse(timeFormat, string(body))
			if err != nil {
				ch <- prometheus.MustNewConstMetric(c.terminationIndicator, prometheus.GaugeValue, 0)

			} else {
				ch <- prometheus.MustNewConstMetric(c.terminationIndicator, prometheus.GaugeValue, 1)
				delta := termtime.Sub(time.Now())
				if delta.Seconds() > 0 {
					ch <- prometheus.MustNewConstMetric(c.terminationTime, prometheus.GaugeValue, delta.Seconds())
				}
			}
		}
	}
}
