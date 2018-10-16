// +build 386 windows,amd64 windows

// Copyright 2018 StreamSets Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package wineventlog

import (
	log "github.com/sirupsen/logrus"
	"syscall"
	"unsafe"
)

type PushWinEventSubscriber struct {
	*BaseWinEventSubscriber
}

func (pwes *PushWinEventSubscriber) Subscribe() error {
	pwes.subscriptionCallback = func(
		action EvtSubscribeNotifyAction,
		userContext unsafe.Pointer,
		eventHandle EventHandle,
	) syscall.Errno {
		var returnStatus syscall.Errno
		switch action {
		case EvtSubscribeActionError:
			log.Errorf("Error Id %d", eventHandle)
			if ErrorEvtQueryResultStale == returnStatus {
				log.Error("The subscription callback was notified that eventHandle records are missing")
			} else {
				log.WithError(syscall.Errno(eventHandle)).Error("The subscription callback received the following Win32 error")
			}
		case EvtSubscribeActionDeliver:
			eventRecord, err := pwes.renderer.RenderEvent(pwes.stageContext, eventHandle, pwes.bookMarkHandle)
			if err == nil {
				pwes.eventsQueue.Put(eventRecord)
			} else {
				log.WithError(err).Errorf("Error rendering from event handle %d", eventHandle)
			}
		}
		return returnStatus
	}
	return pwes.BaseWinEventSubscriber.Subscribe()
}