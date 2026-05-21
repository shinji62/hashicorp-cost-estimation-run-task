package runtask

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/sdk/api"
	"github.com/hashicorp/terraform-run-task-scaffolding-go/internal/sdk/handler"
)

func HandleRequests(task *ScaffoldingRunTask) {
	r := mux.NewRouter()

	task.logger.Println("Registering " + task.config.Path + " route")
	r.HandleFunc(task.config.Path, handleTFCRequestWrapper(task, sendTFCCallbackResponse())).Methods(http.MethodPost)

	task.logger.Println("Registering /healthcheck route")
	r.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		task.logger.Println("/healthcheck called")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{"status": "available"})
		if err != nil {
			return
		}
	}).Methods(http.MethodGet)

	task.logger.Printf("Starting server on port %s", task.config.Addr)
	err := http.ListenAndServe(task.config.Addr, r)
	if err != nil {
		return
	}
}

func handleTFCRequestWrapper(task *ScaffoldingRunTask, original func(http.ResponseWriter, *http.Request, api.Request, *ScaffoldingRunTask, *handler.CallbackBuilder)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		task.logger.Println(task.config.Path + " called")

		// Parse request
		var runTaskReq api.Request
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			task.logger.Println("Error occurred while parsing the request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(reqBody, &runTaskReq)
		if err != nil {
			task.logger.Println("Error occurred while parsing the request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		requestSha := r.Header.Get(handler.HeaderTaskSignature)

		if requestSha != "" && task.config.HmacKey == "" {
			task.logger.Printf("Received a request for %s with a signature but this server cannot validate signed requests\n", r.URL)
			http.Error(w, "Unexpected x-tfc-task-signature header", http.StatusBadRequest)
			return
		}

		if requestSha == "" && task.config.HmacKey != "" {
			task.logger.Printf("Received an unsigned request for %s\n", r.URL)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if requestSha != "" {
			// Calculate expected HMAC
			verified, err := handler.VerifyHMAC(reqBody, []byte(r.Header.Get(handler.HeaderTaskSignature)), []byte(task.config.HmacKey))

			if err != nil {
				task.logger.Println("Unable to verify given HMAC key")
				http.Error(w, "Error verifying signed request", http.StatusInternalServerError)
				return
			}

			if !verified {
				task.logger.Println("Received an unauthorized request")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		if runTaskReq.IsEndpointValidation() {
			task.logger.Println("Successfully validated TFC request")
			w.WriteHeader(http.StatusOK)
			return
		}

		callbackResp, err := task.VerifyRequest(runTaskReq)
		if err != nil {
			task.logger.Println("Error occurred during run task request verification")
			http.Error(w, "Error during run task request verification:"+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get TFC Plan if the task is running in the post-plan or pre-apply stages
		if runTaskReq.Stage == api.PostPlan || runTaskReq.Stage == api.PreApply {
			plan, err := retrieveTFCPlan(runTaskReq, task.logger)

			if err != nil {
				task.logger.Printf("Error occurred while retrieving plan from TFC: %v", err)
				http.Error(w, "Bad Request: "+err.Error(), http.StatusNotFound)
				return
			}
			task.logger.Println("Successfully retrieved plan from TFC")

			callbackResp, err = task.VerifyPlan(runTaskReq, plan)
			if err != nil {
				task.logger.Println("Error occurred while verifying plan")
				http.Error(w, "Error verifying plan:"+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		original(w, r, runTaskReq, task, callbackResp)
	}
}

func sendTFCCallbackResponse() func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {

	return func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {

		respBody, err := cbBuilder.MarshallJSON()
		if err != nil {
			task.logger.Println("Unable to marshall callback response to TFC")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Send PATCH callback response to TFC
		request, err := sendTFCRequest(reqBody.TaskResultCallbackURL, http.MethodPatch, reqBody.AccessToken, respBody)
		if request != nil {
			_ = r.Body.Close()
		}
		if err != nil {
			task.logger.Println("Error occurred while sending the callback response to TFC")
			http.Error(w, "Bad Request:"+err.Error(), http.StatusNotFound)
			return
		}

		task.logger.Println("Sent run task response to TFC")
	}

}

func retrieveTFCPlan(req api.Request, logger *log.Logger) (tfjson.Plan, error) {
	logger.Printf("Retrieving plan from TFC URL: %s", req.PlanJSONAPIURL)

	// Call TFC to get plan
	resp, err := sendTFCRequest(req.PlanJSONAPIURL, "GET", req.AccessToken, nil)
	if err != nil {
		return tfjson.Plan{}, fmt.Errorf("failed to send request to TFC: %w", err)
	}

	var tfPlan tfjson.Plan

	if resp == nil {
		return tfPlan, fmt.Errorf("expected Terraform plan from TFC but received none")
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return tfPlan, fmt.Errorf("TFC returned status %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if err != nil {
		return tfPlan, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Printf("Received plan response body length: %d bytes", len(respBody))

	err = json.Unmarshal(respBody, &tfPlan)
	if err != nil {
		return tfPlan, fmt.Errorf("failed to unmarshal plan JSON: %w", err)
	}

	return tfPlan, nil
}

func sendTFCRequest(url string, method string, accessToken string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	// Required headers to send to TFC
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}
