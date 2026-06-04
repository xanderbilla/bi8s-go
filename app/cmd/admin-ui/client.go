package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) request(method, path string, body io.Reader, headers map[string]string) ([]byte, error) {
	reqURL := c.BaseURL + path
	fmt.Printf("[API Request] %s %s\n", method, reqURL)
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("[API Network Error] %s\n", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		fmt.Printf("[API Error Status] %d: %s\n", resp.StatusCode, string(respBody))
		var env SuccessEnvelope
		if err := json.Unmarshal(respBody, &env); err == nil && env.Error != nil {
			return nil, fmt.Errorf("[%s] %s: %s", env.Error.Code, env.Error.Title, env.Error.Detail)
		}
		return nil, fmt.Errorf("server returned status code %d: %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("[API Response] %d (%d bytes)\n", resp.StatusCode, len(respBody))
	return respBody, nil
}

type FormFile struct {
	FieldName string
	FileName  string
	Reader    io.Reader
}

func (c *Client) multipartRequest(method, path string, fields map[string]string, files []FormFile) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, val := range fields {
		if val != "" {
			err := writer.WriteField(key, val)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, file := range files {
		if file.Reader == nil {
			continue
		}
		part, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(part, file.Reader)
		if err != nil {
			return nil, err
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}

	return c.request(method, path, body, headers)
}

func (c *Client) jsonRequest(method, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.request(method, path, bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
}

func (c *Client) updateContentCore(contentID string, payload map[string]any) ([]byte, error) {
	return c.jsonRequest(http.MethodPatch, fmt.Sprintf("/a/content/%s", contentID), payload)
}

func (c *Client) updatePersonCore(personID string, payload map[string]any) ([]byte, error) {
	return c.jsonRequest(http.MethodPatch, fmt.Sprintf("/a/people/%s", personID), payload)
}

func (c *Client) mutateContentRelation(attributeID, contentID string, add bool) ([]byte, error) {
	action := "remove"
	if add {
		action = "add"
	}
	return c.jsonRequest(http.MethodPost, fmt.Sprintf("/a/content/attributes/%s/%s", attributeID, action), map[string]string{"contentId": contentID})
}

func (c *Client) mutatePersonAttribute(attributeID, personID string, add bool) ([]byte, error) {
	action := "remove"
	if add {
		action = "add"
	}
	return c.jsonRequest(http.MethodPost, fmt.Sprintf("/a/person/attributes/%s/%s", attributeID, action), map[string]string{"personId": personID})
}
