package handler

import (
	"encoding/json"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/sdk/api"
)

// TypeTaskResults is the data type used in run task responses.
const TypeTaskResults = "task-results"

type callbackResponse struct {
	Data api.CallbackData `json:"data"`
}

type CallbackBuilder struct {
	resp callbackResponse
}

func NewCallbackBuilder(status api.TaskStatus) *CallbackBuilder {
	return &CallbackBuilder{
		resp: callbackResponse{
			Data: api.CallbackData{
				Type: TypeTaskResults,
				Attributes: api.Response{
					Status: status,
				},
			},
		},
	}
}

func (cb *CallbackBuilder) WithMessage(message string) *CallbackBuilder {
	cb.resp.Data.Attributes.Message = message
	return cb
}

func (cb *CallbackBuilder) WithURL(url string) *CallbackBuilder {
	cb.resp.Data.Attributes.URL = url
	return cb
}

func (cb *CallbackBuilder) WithOutcomes(outcomes []api.Outcome) *CallbackBuilder {
	// Convert legacy Outcome format to new relationships format
	outcomeData := make([]api.OutcomeData, 0, len(outcomes))
	for _, outcome := range outcomes {
		outcomeData = append(outcomeData, api.OutcomeData{
			Type: "task-result-outcomes",
			Attributes: api.OutcomeAttributes{
				OutcomeID:   outcome.OutcomeID,
				Description: outcome.Description,
				Body:        outcome.Body,
				URL:         outcome.URL,
			},
		})
	}

	if len(outcomeData) > 0 {
		cb.resp.Data.Relationships = &api.CallbackRelationships{
			Outcomes: &api.OutcomesRelationship{
				Data: outcomeData,
			},
		}
	}
	return cb
}

func (cb *CallbackBuilder) AddOutcome(outcome api.Outcome) *CallbackBuilder {
	// Convert single outcome to new relationships format
	outcomeData := api.OutcomeData{
		Type: "task-result-outcomes",
		Attributes: api.OutcomeAttributes{
			OutcomeID:   outcome.OutcomeID,
			Description: outcome.Description,
			Body:        outcome.Body,
			URL:         outcome.URL,
		},
	}

	if cb.resp.Data.Relationships == nil {
		cb.resp.Data.Relationships = &api.CallbackRelationships{
			Outcomes: &api.OutcomesRelationship{
				Data: []api.OutcomeData{outcomeData},
			},
		}
	} else if cb.resp.Data.Relationships.Outcomes == nil {
		cb.resp.Data.Relationships.Outcomes = &api.OutcomesRelationship{
			Data: []api.OutcomeData{outcomeData},
		}
	} else {
		cb.resp.Data.Relationships.Outcomes.Data = append(
			cb.resp.Data.Relationships.Outcomes.Data,
			outcomeData,
		)
	}
	return cb
}

func (cb *CallbackBuilder) MarshallJSON() ([]byte, error) {
	return json.Marshal(cb.resp)
}
