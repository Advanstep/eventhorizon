// Copyright (c) 2014 - Max Persson <max@looplab.se>
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

package eventhorizon

// EventHandler is an interface that all handlers of events should implement.
type EventHandler interface {
	// HandleEvent handles an event.
	HandleEvent(Event)
}

// EventBus is an interface defining an event bus for distributing events.
type EventBus interface {
	// PublishEvent publishes an event on the event bus.
	PublishEvent(Event)
}

// InternalEventBus is an event bus that notifies registered EventHandlers of
// published events.
type InternalEventBus struct {
	eventHandlers  map[string][]EventHandler
	globalHandlers []EventHandler
}

// NewInternalEventBus creates a InternalEventBus.
func NewInternalEventBus() *InternalEventBus {
	b := &InternalEventBus{
		eventHandlers:  make(map[string][]EventHandler),
		globalHandlers: make([]EventHandler, 0),
	}
	return b
}

// PublishEvent publishes an event to all handlers capable of handling it.
func (b *InternalEventBus) PublishEvent(event Event) {
	if handlers, ok := b.eventHandlers[event.EventType()]; ok {
		for _, handler := range handlers {
			handler.HandleEvent(event)
		}
	}

	// Publish to global handlers.
	for _, handler := range b.globalHandlers {
		handler.HandleEvent(event)
	}
}

// AddHandler adds a handler for a specific event.
func (b *InternalEventBus) AddHandler(handler EventHandler, event Event) {
	// Create handler list for new event types.
	if _, ok := b.eventHandlers[event.EventType()]; !ok {
		b.eventHandlers[event.EventType()] = make([]EventHandler, 0)
	}

	// Add handler to event type.
	b.eventHandlers[event.EventType()] = append(b.eventHandlers[event.EventType()], handler)
}

// AddGlobalHandler adds the handler for a specific event.
func (b *InternalEventBus) AddGlobalHandler(handler EventHandler) {
	b.globalHandlers = append(b.globalHandlers, handler)
}
