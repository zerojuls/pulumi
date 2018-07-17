// Copyright 2016-2018, Pulumi Corporation.
//
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

package local

import (
	// "bytes"
	// "fmt"
	// "os"
	// "regexp"
	// "sort"
	// "strings"
	// "time"
	// "unicode"
	// "unicode/utf8"

	// "github.com/pulumi/pulumi/pkg/backend"
	// "github.com/pulumi/pulumi/pkg/diag"
	// "github.com/pulumi/pulumi/pkg/diag/colors"
	// "github.com/pulumi/pulumi/pkg/engine"
	// "github.com/pulumi/pulumi/pkg/resource"
	// "github.com/pulumi/pulumi/pkg/resource/deploy"
	// "github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	// "github.com/pulumi/pulumi/pkg/util/contract"
	// "github.com/docker/docker/pkg/term"
	// "golang.org/x/crypto/ssh/terminal"
)

type NoneInteractiveDisplay struct {
	BaseDisplay

	// A spinner to use to show that we're still doing work even when no output has been
	// printed to the console in a while.
	nonInteractiveSpinner cmdutil.Spinner
}

func createNonInteractiveDisplay() Display {

}

func (display *NoneInteractiveDisplay) processTick() {
	display.baseDisplayProcessTick()

	// Update the spinner to let the user know that that work is still happening.
	display.nonInteractiveSpinner.Tick()
}


func (display *ProgressDisplay) processNormalEvent(event engine.Event) {
	switch event.Type {
	case engine.PreludeEvent:
		// A prelude event can just be printed out directly to the console.
		// Note: we should probably make sure we don't get any prelude events
		// once we start hearing about actual resource events.

		payload := event.Payload.(engine.PreludeEventPayload)
		display.isPreview = payload.IsPreview
		display.writeSimpleMessage(renderPreludeEvent(payload, display.opts))
		return
	case engine.SummaryEvent:
		// keep track of the summar event so that we can display it after all other
		// resource-related events we receive.
		payload := event.Payload.(engine.SummaryEventPayload)
		display.summaryEventPayload = &payload
		return
	case engine.DiagEvent:
		msg := display.renderProgressDiagEvent(event.Payload.(engine.DiagEventPayload), true /*includePrefix:*/)
		if msg == "" {
			return
		}
	case engine.StdoutColorEvent:
		display.handleSystemEvent(event.Payload.(engine.StdoutEventPayload))
		return
	}

	// At this point, all events should relate to resources.
	eventUrn, metadata := getEventUrnAndMetadata(event)
	if eventUrn == "" {
		// If this event has no URN, associate it with the stack. Note that there may not yet be a stack resource, in
		// which case this is a no-op.
		eventUrn = display.stackUrn
	}
	isRootEvent := eventUrn == display.stackUrn

	row := display.getRowForURN(eventUrn, metadata)

	// Don't bother showing certain events (for example, things that are unchanged). However
	// always show the root 'stack' resource so we can indicate that it's still running, and
	// also so we have something to attach unparented diagnostic events to.
	hideRowIfUnnecessary := metadata != nil && !shouldShow(*metadata, display.opts) && !isRootEvent
	if !hideRowIfUnnecessary {
		row.SetHideRowIfUnnecessary(false)
	}

	if event.Type == engine.ResourcePreEvent {
		step := event.Payload.(engine.ResourcePreEventPayload).Metadata
		if step.Op == "" {
			contract.Failf("Got empty op for %s", event.Type)
		}

		row.SetStep(step)
	} else if event.Type == engine.ResourceOutputsEvent {
		// transition the status to done.
		if !isRootEvent {
			row.SetDone()
		}

		step := event.Payload.(engine.ResourceOutputsEventPayload).Metadata
		row.SetStep(step)

		// If we're not in a terminal, we may not want to display this row again: if we're displaying a preview or if
		// this step is a no-op for a custom resource, refreshing this row will simply duplicate its earlier output.
		hasMeaningfulOutput := !display.isPreview && (step.Res == nil || step.Res.Custom && step.Op != deploy.OpSame)
		if !display.isTerminal && !hasMeaningfulOutput {
			return
		}
	} else if event.Type == engine.ResourceOperationFailed {
		row.SetDone()
		row.SetFailed()
	} else if event.Type == engine.DiagEvent {
		// also record this diagnostic so we print it at the end.
		row.RecordDiagEvent(event)
	} else {
		contract.Failf("Unhandled event type '%s'", event.Type)
	}

	if display.isTerminal {
		// if we're in a terminal, then refresh everything so that all our columns line up
		display.refreshAllRowsIfInTerminal()
	} else {
		// otherwise, just print out this single row.
		display.refreshSingleRow("", row, nil)
	}
}