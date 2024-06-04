package telophase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func ValidTelophaseToken(token string) bool {
	if token == "" ||
		token == "ignore" {
		return false
	}

	return true
}

func UpsertAccount(accountID string, accountName string) {
	token := os.Getenv("TELOPHASE_TOKEN")
	if ValidTelophaseToken(token) {
		reqBody, _ := json.Marshal(map[string]string{
			"account_id": accountID,
			"name":       accountName,
		})
		client := &http.Client{}
		req, _ := http.NewRequest("POST", "https://api.telophase.dev/cloudAccount", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error creating account in telophase: %s\n", err)
		}
		if resp.StatusCode != 200 {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("error creating account in telophase: %s\n", string(body))
		}
	}
}

func RecordDeploy(accountID string, accountName string) {
	token := os.Getenv("TELOPHASE_TOKEN")
	if ValidTelophaseToken(token) {
		reqBody, _ := json.Marshal(map[string]string{})
		client := &http.Client{}
		req, _ := http.NewRequest("PATCH", fmt.Sprintf("https://api.telophase.dev/cloudAccount/%s", accountID), bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error creating account in telophase: %s\n", err)
		}
		if resp.StatusCode != 200 {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("error creating account in telophase: %s\n", string(body))
		}
	}
}
