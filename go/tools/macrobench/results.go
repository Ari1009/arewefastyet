/*
 *
 * Copyright 2021 The Vitess Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package macrobench

import (
	"time"

	"github.com/vitessio/arewefastyet/go/exec/metrics"
	"github.com/vitessio/arewefastyet/go/storage"
)

type (
	// QPS represents the QPS table. This table contains the raw
	// results of a macro benchmark.
	QPS struct {
		ID     int
		RefID  int
		Total  float64 `json:"total"`
		Reads  float64 `json:"reads"`
		Writes float64 `json:"writes"`
		Other  float64 `json:"other"`
	}

	// Result represents both OLTP and TPCC tables.
	// The two tables share the same schema and can thus be grouped
	// under an unique go struct.
	Result struct {
		ID         int
		Queries    int     `json:"queries"`
		QPS        QPS     `json:"qps"`
		TPS        float64 `json:"tps"`
		Latency    float64 `json:"latency"`
		Errors     float64 `json:"errors"`
		Reconnects float64 `json:"reconnects"`
		Time       int     `json:"time"`
		Threads    float64 `json:"threads"`
	}

	qpsAsSlice struct {
		total  []float64
		reads  []float64
		writes []float64
		other  []float64
	}

	metricsAsSlice struct {
		totalComponentsCPUTime []float64
		componentsCPUTime      map[string][]float64

		totalComponentsMemStatsAllocBytes []float64
		componentsMemStatsAllocBytes      map[string][]float64
	}

	resultAsSlice struct {
		qps qpsAsSlice

		tps        []float64
		latency    []float64
		errors     []float64
		reconnects []float64
		time       []int
		threads    []float64

		metrics metricsAsSlice
	}

	// BenchmarkID is used to identify a macro benchmark using its database's ID, the
	// source from which the benchmark was triggered and its creation date.
	BenchmarkID struct {
		ID        int
		Source    string
		CreatedAt *time.Time
		ExecUUID  string
	}

	// Details represents the entire macro benchmark and its sub
	// components. It has a BenchmarkID (ID, creation date, source of the benchmark),
	// the git reference that was used, and its results represented by a Result.
	// This struct encapsulates the "benchmark", "qps" and ("OLTP" or "TPCC") database tables.
	Details struct {
		BenchmarkID

		// refers to commit
		GitRef  string
		Result  Result
		Metrics metrics.ExecutionMetrics
	}

	BenchmarkResults struct {
		Results ResultsArray
		Metrics metrics.ExecutionMetricsArray
	}

	ResultsArray []Result
	DetailsArray []Details

	DailySummary struct {
		CreatedAt *time.Time
		QPSTotal  float64
	}
)

func (br BenchmarkResults) asSlice() resultAsSlice {
	s := br.Results.resultsArrayToSlice()
	s.metrics = metricsToSlice(br.Metrics)
	return s
}

func metricsToSlice(metrics metrics.ExecutionMetricsArray) metricsAsSlice {
	var s metricsAsSlice
	s.componentsCPUTime = make(map[string][]float64)
	s.componentsMemStatsAllocBytes = make(map[string][]float64)
	for _, metricRow := range metrics {
		s.totalComponentsCPUTime = append(s.totalComponentsCPUTime, metricRow.TotalComponentsCPUTime)
		for name, value := range metricRow.ComponentsCPUTime {
			s.componentsCPUTime[name] = append(s.componentsCPUTime[name], value)
		}

		s.totalComponentsMemStatsAllocBytes = append(s.totalComponentsMemStatsAllocBytes, metricRow.TotalComponentsMemStatsAllocBytes)
		for name, value := range metricRow.ComponentsMemStatsAllocBytes {
			s.componentsMemStatsAllocBytes[name] = append(s.componentsMemStatsAllocBytes[name], value)
		}
	}
	return s
}

func (mrs ResultsArray) resultsArrayToSlice() resultAsSlice {
	var ras resultAsSlice
	for _, mr := range mrs {
		ras.qps.total = append(ras.qps.total, mr.QPS.Total)
		ras.qps.reads = append(ras.qps.reads, mr.QPS.Reads)
		ras.qps.writes = append(ras.qps.writes, mr.QPS.Writes)
		ras.qps.other = append(ras.qps.other, mr.QPS.Other)
		ras.tps = append(ras.tps, mr.TPS)
		ras.latency = append(ras.latency, mr.Latency)
		ras.errors = append(ras.errors, mr.Errors)
		ras.reconnects = append(ras.reconnects, mr.Reconnects)
		ras.time = append(ras.time, mr.Time)
		ras.threads = append(ras.threads, mr.Threads)
	}
	return ras
}

func Compare(client storage.SQLClient, old, new string, types []string, planner PlannerVersion) (map[string]StatisticalCompareResults, error) {
	results := make(map[string]StatisticalCompareResults, len(types))
	for _, macroType := range types {
		leftResult, err := getBenchmarkResults(client, macroType, old, planner)
		if err != nil {
			return nil, err
		}

		rightResult, err := getBenchmarkResults(client, macroType, new, planner)
		if err != nil {
			return nil, err
		}

		leftResultsAsSlice := leftResult.asSlice()
		rightResultsAsSlice := rightResult.asSlice()

		scr := performAnalysis(leftResultsAsSlice, rightResultsAsSlice)
		results[macroType] = scr
	}
	return results, nil
}

func Search(client storage.SQLClient, sha string, types []string, planner PlannerVersion) (map[string]StatisticalSingleResult, error) {
	results := make(map[string]StatisticalSingleResult, len(types))
	for _, macroType := range types {
		result, err := getBenchmarkResults(client, macroType, sha, planner)
		if err != nil {
			return nil, err
		}
		resultSlice := result.asSlice()
		ssr := StatisticalSingleResult{
			ComponentsCPUTime:            map[string]StatisticalSummary{},
			ComponentsMemStatsAllocBytes: map[string]StatisticalSummary{},
		}

		ssr.TotalQPS, _ = getSummary(resultSlice.qps.total)
		ssr.ReadsQPS, _ = getSummary(resultSlice.qps.reads)
		ssr.WritesQPS, _ = getSummary(resultSlice.qps.writes)
		ssr.OtherQPS, _ = getSummary(resultSlice.qps.other)

		ssr.TPS, _ = getSummary(resultSlice.tps)
		ssr.Latency, _ = getSummary(resultSlice.latency)
		ssr.Errors, _ = getSummary(resultSlice.errors)

		ssr.TotalComponentsCPUTime, _ = getSummary(resultSlice.metrics.totalComponentsCPUTime)
		for name, value := range resultSlice.metrics.componentsCPUTime {
			ssr.ComponentsCPUTime[name], _ = getSummary(value)
		}

		ssr.TotalComponentsMemStatsAllocBytes, _ = getSummary(resultSlice.metrics.totalComponentsMemStatsAllocBytes)
		for name, value := range resultSlice.metrics.componentsMemStatsAllocBytes {
			ssr.ComponentsMemStatsAllocBytes[name], _ = getSummary(value)
		}
		results[macroType] = ssr
	}
	return results, nil
}

func getBenchmarkResults(client storage.SQLClient, macroType, gitSHA string, planner PlannerVersion) (BenchmarkResults, error) {
	results, err := getResultsForGitRefAndPlanner(macroType, gitSHA, planner, client)
	if err != nil {
		return BenchmarkResults{}, err
	}

	if len(results) == 0 {
		return BenchmarkResults{}, nil
	}

	var br BenchmarkResults
	for _, result := range results {
		br.Results = append(br.Results, result.Result)

		metricsResult, err := metrics.GetExecutionMetricsSQL(client, result.ExecUUID)
		if err != nil {
			return BenchmarkResults{}, err
		}
		br.Metrics = append(br.Metrics, metricsResult)
	}
	return br, nil
}

func getBenchmarkResultsLastXDays(client storage.SQLClient, macroType, sha string, planner PlannerVersion, days int) (BenchmarkResults, error) {
	results, err := GetResultsForLastDays(macroType, sha, planner, days, client)
	if err != nil {
		return BenchmarkResults{}, err
	}

	if len(results) == 0 {
		return BenchmarkResults{}, nil
	}

	var br BenchmarkResults
	for _, result := range results {
		br.Results = append(br.Results, result.Result)

		metricsResult, err := metrics.GetExecutionMetricsSQL(client, result.ExecUUID)
		if err != nil {
			return BenchmarkResults{}, err
		}
		br.Metrics = append(br.Metrics, metricsResult)
	}
	return br, nil
}
