package collect

import (
	"context"
	"fmt"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// one series to fetch
type Query struct {
	Namespace  string
	Metric     string
	Dimension  string // dimension name, e.g. "NatGatewayId"
	ResourceID string // dimension value, e.g. "nat-123456"
}

const (
	maxQueriesPerCall = 500
	periodSeconds     = 86400 // 1 day
)

// MetricsSum fetches every query in as few calls as possible
// A series with no datapoints is left out of result. So, MetricSet.Sum reports it as unknown rather than zero
func MetricsSum(ctx context.Context, api awsapi.CloudWatchAPI, now time.Time, window time.Duration, queries []Query) (zombie.MetricSet, error) {
	ms := zombie.NewMetricSet(window)
	if len(queries) == 0 {
		return ms, nil
	}

	start := now.Add(-window)

	for base := 0; base < len(queries); base += maxQueriesPerCall {
		batch := queries[base:min(base+maxQueriesPerCall, len(queries))]

		dq := make([]cwtypes.MetricDataQuery, len(batch))
		byID := make(map[string]Query, len(batch))

		for i, q := range batch {
			id := fmt.Sprintf("m%d", i)
			byID[id] = q

			dq = append(dq, cwtypes.MetricDataQuery{
				Id: aws.String(id),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String(q.Namespace),
						MetricName: aws.String(q.Metric),
						Dimensions: []cwtypes.Dimension{
							{
								Name:  aws.String(q.Dimension),
								Value: aws.String(q.ResourceID),
							},
						},
					},
					Period: aws.Int32(periodSeconds),
					Stat:   aws.String("Sum"),
				},
			})
		}

		// one resource's datapoinst can span pages, so accumulate before storing
		totals := map[string]float64{}
		seen := map[string]bool{}

		var token *string
		for {
			resp, err := api.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
				MetricDataQueries: dq,
				StartTime:         aws.Time(start),
				EndTime:           aws.Time(now),
				NextToken:         token,
			})
			if err != nil {
				return zombie.MetricSet{}, fmt.Errorf("cloudwatch:GetMetricData: %w", err)
			}
			for _, r := range resp.MetricDataResults {
				id := aws.ToString(r.Id)
				if _, ok := byID[id]; !ok || len(r.Values) == 0 {
					continue
				}
				seen[id] = true
				for _, v := range r.Values {
					totals[id] += v
				}
			}
			if resp.NextToken == nil {
				break
			}
			token = resp.NextToken
		}

		for id := range seen {
			q := byID[id]
			ms.Set(zombie.MetricKey{
				NameSpace:  q.Namespace,
				Metric:     q.Metric,
				ResourceID: q.ResourceID,
			}, totals[id])
		}

	}

	return ms, nil
}
