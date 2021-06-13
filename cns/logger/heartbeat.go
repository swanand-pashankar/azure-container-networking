// Copyright 2018 Microsoft. All rights reserved.
// MIT License

package logger

import (
	"context"
	"reflect"
	"regexp"
	"time"

	"github.com/Azure/azure-container-networking/aitelemetry"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
	"github.com/Azure/azure-container-networking/telemetry"
	"github.com/google/uuid"
)

const (
	// CNSTelemetryFile - telemetry file path.
	cnsTelemetryFile                = platform.CNSRuntimePath + "AzureCNSTelemetry.json"
	errorcodePrefix                 = 5
	heartbeatIntervalInMinutes      = 30
	retryWaitTimeInSeconds          = 60
	telemetryNumRetries             = 5
	telemetryWaitTimeInMilliseconds = 200
)

var codeRegex = regexp.MustCompile(`Code:(\w*)`)

func SendHeartBeat(ctx context.Context, heartbeatIntervalInMins int) {
	heartbeat := time.NewTicker(time.Minute * time.Duration(heartbeatIntervalInMins)).C
	metric := aitelemetry.Metric{
		Name: HeartBeatMetricStr,
		// This signifies 1 heartbeat is sent. Sum of this metric will give us number of heartbeats received
		Value:            1.0,
		CustomDimensions: make(map[string]string),
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat:
			SendMetric(metric)
		}
	}
}

// SendCnsTelemetry - handles cns telemetry reports
func SendToTelemetryService(ctx context.Context, reports <-chan interface{}) {

CONNECT:
	tb := telemetry.NewTelemetryBuffer("")
	tb.ConnectToTelemetryService(telemetryNumRetries, telemetryWaitTimeInMilliseconds)

	if tb.Connected {

		reportMgr := telemetry.ReportManager{
			ContentType: telemetry.ContentType,
			Report:      &telemetry.CNSReport{},
		}

		reportMgr.GetReportState(cnsTelemetryFile)
		reportMgr.GetKernelVersion()

		for {
			select {
			case <-ctx.Done():
				tb.Close()
				return
			case msg := <-reports:
				codeStr := codeRegex.FindString(msg.(string))
				if len(codeStr) > errorcodePrefix {
					reflect.ValueOf(reportMgr.Report).Elem().FieldByName("Errorcode").SetString(codeStr[errorcodePrefix:])
				}
				reflect.ValueOf(reportMgr.Report).Elem().FieldByName("EventMessage").SetString(msg.(string))
			}

			reflect.ValueOf(reportMgr.Report).Elem().FieldByName("Timestamp").SetString(time.Now().UTC().String())
			if id, err := uuid.NewUUID(); err == nil {
				reflect.ValueOf(reportMgr.Report).Elem().FieldByName("UUID").SetString(id.String())
			}

			if !reportMgr.GetReportState(cnsTelemetryFile) {
				reportMgr.SetReportState(cnsTelemetryFile)
			}

			report, err := reportMgr.ReportToBytes()
			if err == nil {
				// If write fails, try to re-establish connections as server/client
				if _, err = tb.Write(report); err != nil {
					log.Logf("[CNS-Telemetry] Telemetry write failed: %v", err)
					tb.Close()
					goto CONNECT
				}
			}
		}
	}
}
