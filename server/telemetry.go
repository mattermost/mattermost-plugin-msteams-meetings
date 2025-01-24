// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

type TelemetrySource string

const (
	telemetryStartSourceWebapp  TelemetrySource = "webapp"
	telemetryStartSourceCommand TelemetrySource = "command"
)

func (p *Plugin) trackConnect(userID string) {
	_ = p.tracker.TrackUserEvent("connect", userID, map[string]interface{}{})
}

func (p *Plugin) trackDisconnect(userID string) {
	_ = p.tracker.TrackUserEvent("disconnect", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingStart(userID string, source TelemetrySource) {
	_ = p.tracker.TrackUserEvent("meeting_started", userID, map[string]interface{}{
		"source": source,
	})
}

func (p *Plugin) trackMeetingDuplication(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_duplicated", userID, map[string]interface{}{})
}

func (p *Plugin) trackMeetingForced(userID string) {
	_ = p.tracker.TrackUserEvent("meeting_forced", userID, map[string]interface{}{})
}
