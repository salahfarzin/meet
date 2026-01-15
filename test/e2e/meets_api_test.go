package e2e

import (
	"net/http"
	"testing"
	"time"
)

// Meet represents a meet object in API responses
type Meet struct {
	UUID             string   `json:"uuid,omitempty"`
	OrganizerUUID    string   `json:"organizer_uuid,omitempty"`
	PriceUUID        string   `json:"price_uuid,omitempty"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	Color            string   `json:"color,omitempty"`
	Start            string   `json:"start"`
	End              string   `json:"end"`
	ParticipantUUIDs []string `json:"participant_uuids,omitempty"`
	Type             string   `json:"type,omitempty"` // API returns enum names like "VIDEO_CALL"
	BookedAt         *string  `json:"booked_at,omitempty"`
}

// CreateMeetResponse represents the response from creating a meet
type CreateMeetResponse struct {
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	Meet Meet `json:"meet"`
}

// GetOneResponse represents the response from getting a single meet
type GetOneResponse struct {
	Meet Meet `json:"meet"`
}

// GetAllResponse represents the response from getting all meets
type GetAllResponse struct {
	Meets []Meet `json:"meets"`
}

// UpdateMeetResponse represents the response from updating a meet
type UpdateMeetResponse struct {
	Meet Meet `json:"meet"`
}

func TestMeetsAPI_CreateMeet(t *testing.T) {
	config := NewTestConfig()

	// Test data
	now := time.Now().UTC()
	startTime := now.Add(24 * time.Hour).Format(time.RFC3339)
	endTime := now.Add(25 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, resp *APIResponse)
	}{
		{
			name: "Create meet successfully",
			payload: map[string]interface{}{
				"title":       "Team Standup",
				"description": "Daily team standup meeting",
				"start":       startTime,
				"end":         endTime,
				"color":       "#FF5733",
				"type":        4, // VIDEO_CALL
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *APIResponse) {
				var result CreateMeetResponse
				resp.DecodeJSON(t, &result)

				if result.Meet.UUID == "" {
					t.Error("Expected meet UUID to be set")
				}
				if result.Meet.Title != "Team Standup" {
					t.Errorf("Expected title 'Team Standup', got '%s'", result.Meet.Title)
				}
				if result.Status.Code != 0 {
					t.Errorf("Expected status code 0, got %d", result.Status.Code)
				}
			},
		},
		{
			name: "Create meet with missing title",
			payload: map[string]interface{}{
				"start": startTime,
				"end":   endTime,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp *APIResponse) {
				// Should contain error about missing title
				if len(resp.Body) == 0 {
					t.Error("Expected error response body")
				}
			},
		},
		{
			name: "Create meet with missing start time",
			payload: map[string]interface{}{
				"title": "Test Meet",
				"end":   endTime,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  func(t *testing.T, resp *APIResponse) {},
		},
		{
			name: "Create meet with invalid time format",
			payload: map[string]interface{}{
				"title": "Test Meet",
				"start": "invalid-time",
				"end":   endTime,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  func(t *testing.T, resp *APIResponse) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := config.DoRequest(t, APIRequest{
				Method:  http.MethodPost,
				Path:    "/meets",
				Body:    tt.payload,
				Headers: GetAuthHeaders(),
			})

			resp.AssertStatusCode(t, tt.expectedStatus)
			tt.checkResponse(t, resp)
		})
	}
}

func TestMeetsAPI_GetOneMeet(t *testing.T) {
	config := NewTestConfig()

	// First, create a meet to retrieve
	now := time.Now().UTC()
	createResp := config.DoRequest(t, APIRequest{
		Method: http.MethodPost,
		Path:   "/meets",
		Body: map[string]interface{}{
			"title": "Test Meet for Retrieval",
			"start": now.Add(24 * time.Hour).Format(time.RFC3339),
			"end":   now.Add(25 * time.Hour).Format(time.RFC3339),
		},
		Headers: GetAuthHeaders(),
	})

	var createResult CreateMeetResponse
	createResp.DecodeJSON(t, &createResult)
	meetUUID := createResult.Meet.UUID

	tests := []struct {
		name           string
		uuid           string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *APIResponse)
	}{
		{
			name:           "Get existing meet",
			uuid:           meetUUID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *APIResponse) {
				var result GetOneResponse
				resp.DecodeJSON(t, &result)

				if result.Meet.UUID != meetUUID {
					t.Errorf("Expected UUID %s, got %s", meetUUID, result.Meet.UUID)
				}
				if result.Meet.Title != "Test Meet for Retrieval" {
					t.Errorf("Expected title 'Test Meet for Retrieval', got '%s'", result.Meet.Title)
				}
			},
		},
		{
			name:           "Get non-existent meet",
			uuid:           "non-existent-uuid",
			expectedStatus: http.StatusNotFound,
			checkResponse:  func(t *testing.T, resp *APIResponse) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := config.DoRequest(t, APIRequest{
				Method:  http.MethodGet,
				Path:    "/meets/" + tt.uuid,
				Headers: GetAuthHeaders(),
			})

			resp.AssertStatusCode(t, tt.expectedStatus)
			tt.checkResponse(t, resp)
		})
	}
}

func TestMeetsAPI_GetAllMeets(t *testing.T) {
	config := NewTestConfig()

	// Create a few meets first
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		config.DoRequest(t, APIRequest{
			Method: http.MethodPost,
			Path:   "/meets",
			Body: map[string]interface{}{
				"title": "Test Meet " + string(rune('A'+i)),
				"start": now.Add(time.Duration(24*(i+1)) * time.Hour).Format(time.RFC3339),
				"end":   now.Add(time.Duration(24*(i+1)+1) * time.Hour).Format(time.RFC3339),
			},
			Headers: GetAuthHeaders(),
		})
	}

	t.Run("Get all meets", func(t *testing.T) {
		resp := config.DoRequest(t, APIRequest{
			Method:  http.MethodGet,
			Path:    "/meets",
			Headers: GetAuthHeaders(),
		})

		resp.AssertStatusCode(t, http.StatusOK)

		var result GetAllResponse
		resp.DecodeJSON(t, &result)

		if len(result.Meets) < 3 {
			t.Errorf("Expected at least 3 meets, got %d", len(result.Meets))
		}
	})

	t.Run("Get meets with date range filter", func(t *testing.T) {
		from := now.Add(24 * time.Hour).Format(time.RFC3339)
		to := now.Add(48 * time.Hour).Format(time.RFC3339)

		resp := config.DoRequest(t, APIRequest{
			Method:  http.MethodGet,
			Path:    "/meets?from=" + from + "&to=" + to,
			Headers: GetAuthHeaders(),
		})

		resp.AssertStatusCode(t, http.StatusOK)

		var result GetAllResponse
		resp.DecodeJSON(t, &result)

		// Should have filtered results
		if len(result.Meets) == 0 {
			t.Error("Expected some meets in the date range")
		}
	})
}

func TestMeetsAPI_UpdateMeet(t *testing.T) {
	config := NewTestConfig()

	// First, create a meet to update
	now := time.Now().UTC()
	createResp := config.DoRequest(t, APIRequest{
		Method: http.MethodPost,
		Path:   "/meets",
		Body: map[string]interface{}{
			"title": "Original Title",
			"start": now.Add(24 * time.Hour).Format(time.RFC3339),
			"end":   now.Add(25 * time.Hour).Format(time.RFC3339),
		},
		Headers: GetAuthHeaders(),
	})

	var createResult CreateMeetResponse
	createResp.DecodeJSON(t, &createResult)
	meetUUID := createResult.Meet.UUID

	tests := []struct {
		name           string
		uuid           string
		payload        map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, resp *APIResponse)
	}{
		{
			name: "Update meet successfully",
			uuid: meetUUID,
			payload: map[string]interface{}{
				"title":       "Updated Title",
				"description": "Updated description",
				"start":       now.Add(24 * time.Hour).Format(time.RFC3339),
				"end":         now.Add(25 * time.Hour).Format(time.RFC3339),
				"color":       "#00FF00",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *APIResponse) {
				var result UpdateMeetResponse
				resp.DecodeJSON(t, &result)

				if result.Meet.Title != "Updated Title" {
					t.Errorf("Expected title 'Updated Title', got '%s'", result.Meet.Title)
				}
				if result.Meet.Description != "Updated description" {
					t.Errorf("Expected description 'Updated description', got '%s'", result.Meet.Description)
				}
			},
		},
		{
			name: "Update non-existent meet",
			uuid: "non-existent-uuid",
			payload: map[string]interface{}{
				"title": "Updated Title",
				"start": now.Add(24 * time.Hour).Format(time.RFC3339),
				"end":   now.Add(25 * time.Hour).Format(time.RFC3339),
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse:  func(t *testing.T, resp *APIResponse) {},
		},
		{
			name: "Update with missing required fields",
			uuid: meetUUID,
			payload: map[string]interface{}{
				"description": "Only description",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  func(t *testing.T, resp *APIResponse) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := config.DoRequest(t, APIRequest{
				Method:  http.MethodPut,
				Path:    "/meets/" + tt.uuid,
				Body:    tt.payload,
				Headers: GetAuthHeaders(),
			})

			resp.AssertStatusCode(t, tt.expectedStatus)
			tt.checkResponse(t, resp)
		})
	}
}

func TestMeetsAPI_DeleteMeet(t *testing.T) {
	config := NewTestConfig()

	// First, create a meet to delete
	now := time.Now().UTC()
	createResp := config.DoRequest(t, APIRequest{
		Method: http.MethodPost,
		Path:   "/meets",
		Body: map[string]interface{}{
			"title": "Meet to Delete",
			"start": now.Add(24 * time.Hour).Format(time.RFC3339),
			"end":   now.Add(25 * time.Hour).Format(time.RFC3339),
		},
		Headers: GetAuthHeaders(),
	})

	var createResult CreateMeetResponse
	createResp.DecodeJSON(t, &createResult)
	meetUUID := createResult.Meet.UUID

	tests := []struct {
		name           string
		uuid           string
		expectedStatus int
	}{
		{
			name:           "Delete existing meet",
			uuid:           meetUUID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Delete already deleted meet",
			uuid:           meetUUID,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := config.DoRequest(t, APIRequest{
				Method:  http.MethodDelete,
				Path:    "/meets/" + tt.uuid,
				Headers: GetAuthHeaders(),
			})

			resp.AssertStatusCode(t, tt.expectedStatus)
		})
	}
}

func TestMeetsAPI_FullCRUDFlow(t *testing.T) {
	config := NewTestConfig()
	now := time.Now().UTC()

	// 1. Create a meet
	t.Log("Step 1: Creating a meet")
	createResp := config.DoRequest(t, APIRequest{
		Method: http.MethodPost,
		Path:   "/meets",
		Body: map[string]interface{}{
			"title":       "Full CRUD Test Meet",
			"description": "Testing complete CRUD flow",
			"start":       now.Add(24 * time.Hour).Format(time.RFC3339),
			"end":         now.Add(25 * time.Hour).Format(time.RFC3339),
			"color":       "#FF5733",
			"type":        4,
		},
		Headers: GetAuthHeaders(),
	})
	createResp.AssertStatusCode(t, http.StatusOK)

	var createResult CreateMeetResponse
	createResp.DecodeJSON(t, &createResult)
	meetUUID := createResult.Meet.UUID

	if meetUUID == "" {
		t.Fatal("Failed to create meet: UUID is empty")
	}
	t.Logf("Created meet with UUID: %s", meetUUID)

	// 2. Read the meet
	t.Log("Step 2: Reading the created meet")
	getResp := config.DoRequest(t, APIRequest{
		Method:  http.MethodGet,
		Path:    "/meets/" + meetUUID,
		Headers: GetAuthHeaders(),
	})
	getResp.AssertStatusCode(t, http.StatusOK)

	var getResult GetOneResponse
	getResp.DecodeJSON(t, &getResult)

	if getResult.Meet.Title != "Full CRUD Test Meet" {
		t.Errorf("Expected title 'Full CRUD Test Meet', got '%s'", getResult.Meet.Title)
	}

	// 3. Update the meet
	t.Log("Step 3: Updating the meet")
	updateResp := config.DoRequest(t, APIRequest{
		Method: http.MethodPut,
		Path:   "/meets/" + meetUUID,
		Body: map[string]interface{}{
			"title":       "Updated CRUD Test Meet",
			"description": "Updated description",
			"start":       now.Add(24 * time.Hour).Format(time.RFC3339),
			"end":         now.Add(25 * time.Hour).Format(time.RFC3339),
			"color":       "#00FF00",
		},
		Headers: GetAuthHeaders(),
	})
	updateResp.AssertStatusCode(t, http.StatusOK)

	var updateResult UpdateMeetResponse
	updateResp.DecodeJSON(t, &updateResult)

	if updateResult.Meet.Title != "Updated CRUD Test Meet" {
		t.Errorf("Expected updated title, got '%s'", updateResult.Meet.Title)
	}

	// 4. Verify the update
	t.Log("Step 4: Verifying the update")
	verifyResp := config.DoRequest(t, APIRequest{
		Method:  http.MethodGet,
		Path:    "/meets/" + meetUUID,
		Headers: GetAuthHeaders(),
	})
	verifyResp.AssertStatusCode(t, http.StatusOK)

	var verifyResult GetOneResponse
	verifyResp.DecodeJSON(t, &verifyResult)

	if verifyResult.Meet.Title != "Updated CRUD Test Meet" {
		t.Error("Update was not persisted")
	}

	// 5. Delete the meet
	t.Log("Step 5: Deleting the meet")
	deleteResp := config.DoRequest(t, APIRequest{
		Method:  http.MethodDelete,
		Path:    "/meets/" + meetUUID,
		Headers: GetAuthHeaders(),
	})
	deleteResp.AssertStatusCode(t, http.StatusOK)

	// 6. Verify deletion
	t.Log("Step 6: Verifying deletion")
	deletedResp := config.DoRequest(t, APIRequest{
		Method:  http.MethodGet,
		Path:    "/meets/" + meetUUID,
		Headers: GetAuthHeaders(),
	})
	deletedResp.AssertStatusCode(t, http.StatusNotFound)

	t.Log("✅ Full CRUD flow completed successfully")
}
