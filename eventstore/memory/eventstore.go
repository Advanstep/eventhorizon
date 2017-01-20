// Copyright (c) 2014 - Max Ekman <max@looplab.se>
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

package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
)

// ErrCouldNotSaveAggregate is when an aggregate could not be saved.
var ErrCouldNotSaveAggregate = errors.New("could not save aggregate")

// EventStore implements EventStore as an in memory structure.
type EventStore struct {
	// The outer map is with namespace as key, the inner with aggregate ID.
	db   map[string]map[eh.UUID]aggregateRecord
	dbMu sync.RWMutex
}

// NewEventStore creates a new EventStore.
func NewEventStore() *EventStore {
	s := &EventStore{
		db: map[string]map[eh.UUID]aggregateRecord{},
	}
	return s
}

// Save appends all events in the event stream to the memory store.
func (s *EventStore) Save(ctx context.Context, events []eh.Event, originalVersion int) error {
	if len(events) == 0 {
		return eh.EventStoreError{
			Err:       eh.ErrNoEventsToAppend,
			Namespace: eh.Namespace(ctx),
		}
	}

	// Build all event records, with incrementing versions starting from the
	// original aggregate version.
	dbEvents := make([]dbEvent, len(events))
	aggregateID := events[0].AggregateID()
	version := originalVersion
	for i, event := range events {
		// Only accept events belonging to the same aggregate.
		if event.AggregateID() != aggregateID {
			return eh.EventStoreError{
				Err:       eh.ErrInvalidEvent,
				Namespace: eh.Namespace(ctx),
			}
		}

		// Only accept events that apply to the correct aggregate version.
		if event.Version() != version+1 {
			return eh.EventStoreError{
				Err:       eh.ErrIncorrectEventVersion,
				Namespace: eh.Namespace(ctx),
			}
		}

		// Create the event record with timestamp.
		dbEvents[i] = dbEvent{
			EventType:     event.EventType(),
			Data:          event.Data(),
			Timestamp:     event.Timestamp(),
			AggregateType: event.AggregateType(),
			AggregateID:   event.AggregateID(),
			Version:       event.Version(),
		}

		version++
	}

	ns := s.namespace(ctx)

	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	// Either insert a new aggregate or append to an existing.
	if originalVersion == 0 {
		aggregate := aggregateRecord{
			AggregateID: aggregateID,
			Version:     len(dbEvents),
			Events:      dbEvents,
		}

		s.db[ns][aggregateID] = aggregate
	} else {
		// Increment aggregate version on insert of new event record, and
		// only insert if version of aggregate is matching (ie not changed
		// since loading the aggregate).
		if aggregate, ok := s.db[ns][aggregateID]; ok {
			if aggregate.Version != originalVersion {
				return eh.EventStoreError{
					Err:       ErrCouldNotSaveAggregate,
					Namespace: eh.Namespace(ctx),
				}
			}

			aggregate.Version += len(dbEvents)
			aggregate.Events = append(aggregate.Events, dbEvents...)

			s.db[ns][aggregateID] = aggregate
		}
	}

	return nil
}

// Load loads all events for the aggregate id from the memory store.
// Returns ErrNoEventsFound if no events can be found.
func (s *EventStore) Load(ctx context.Context, aggregateType eh.AggregateType, id eh.UUID) ([]eh.Event, error) {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()

	// Ensure that the namespace exists.
	s.dbMu.RUnlock()
	ns := s.namespace(ctx)
	s.dbMu.RLock()

	aggregate, ok := s.db[ns][id]
	if !ok {
		return []eh.Event{}, nil
	}

	events := make([]eh.Event, len(aggregate.Events))
	for i, dbEvent := range aggregate.Events {
		events[i] = event{dbEvent: dbEvent}
	}

	return events, nil
}

// Helper to get the namespace and ensure that its data exists.
func (s *EventStore) namespace(ctx context.Context) string {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	ns := eh.Namespace(ctx)
	if _, ok := s.db[ns]; !ok {
		s.db[ns] = map[eh.UUID]aggregateRecord{}
	}
	return ns
}

type aggregateRecord struct {
	AggregateID eh.UUID
	Version     int
	Events      []dbEvent
	// Snapshot    eh.Aggregate
}

// dbEvent is the internal event record for the memory event store.
type dbEvent struct {
	EventType     eh.EventType
	Data          eh.EventData
	Timestamp     time.Time
	AggregateType eh.AggregateType
	AggregateID   eh.UUID
	Version       int
}

// event is the private implementation of the eventhorizon.Event interface
// for a memory event store.
type event struct {
	dbEvent
}

// EventType implements the EventType method of the eventhorizon.Event interface.
func (e event) EventType() eh.EventType {
	return e.dbEvent.EventType
}

// Data implements the Data method of the eventhorizon.Event interface.
func (e event) Data() eh.EventData {
	return e.dbEvent.Data
}

// Timestamp implements the Timestamp method of the eventhorizon.Event interface.
func (e event) Timestamp() time.Time {
	return e.dbEvent.Timestamp
}

// AggregateType implements the AggregateType method of the eventhorizon.Event interface.
func (e event) AggregateType() eh.AggregateType {
	return e.dbEvent.AggregateType
}

// AggrgateID implements the AggrgateID method of the eventhorizon.Event interface.
func (e event) AggregateID() eh.UUID {
	return e.dbEvent.AggregateID
}

// Version implements the Version method of the eventhorizon.Event interface.
func (e event) Version() int {
	return e.dbEvent.Version
}

// String implements the String method of the eventhorizon.Event interface.
func (e event) String() string {
	return fmt.Sprintf("%s@%d", e.dbEvent.EventType, e.dbEvent.Version)
}
