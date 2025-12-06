// file registry.go interacts with the upstream docker registry
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// CatalogResponse represents the response from /v2/_catalog
type CatalogResponse struct {
	Repositories []string `json:"repositories"`
}

// TagsResponse represents the response from /v2/<n>/tags/list
type TagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// imageInfo holds repository and its tags
type imageInfo struct {
	Repository string
	Tags       []string
}

// registryConfig holds configuration for connecting to a registry
type registryConfig struct {
	URL      string
	Scheme   string
	Username string
	Password string
	Insecure bool // Set to true to skip TLS verification
}

// listAllImages queries a Docker registry and returns all repositories with their tags
func listAllImages(config registryConfig) ([]imageInfo, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get list of all repositories from catalog (with pagination)
	repositories, err := getCatalogPaginated(client, config)
	if err != nil {
		return nil, err
	}

	// Get tags for each repository
	var images []imageInfo
	for _, repo := range repositories {
		tags, err := getTagsPaginated(client, config, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get tags for %s: %w", repo, err)
		}

		images = append(images, imageInfo{
			Repository: repo,
			Tags:       tags,
		})
	}

	return images, nil
}

// getCatalogPaginated retrieves all repositories from the registry using pagination
func getCatalogPaginated(client *http.Client, config registryConfig) ([]string, error) {
	var allRepos []string
	nextURL := fmt.Sprintf("%s://%s/v2/_catalog?n=100", config.Scheme, config.URL)

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create catalog request: %w", err)
		}

		if config.Username != "" && config.Password != "" {
			req.SetBasicAuth(config.Username, config.Password)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to query catalog: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("catalog request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var catalog CatalogResponse
		if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode catalog response: %w", err)
		}
		resp.Body.Close()

		allRepos = append(allRepos, catalog.Repositories...)

		// Check for next page in Link header
		nextURL = getNextPageURL(resp.Header.Get("Link"), config.Scheme, config.URL)
	}

	return allRepos, nil
}

// getTagsPaginated retrieves all tags for a specific repository using pagination
func getTagsPaginated(client *http.Client, config registryConfig, repo string) ([]string, error) {
	var allTags []string
	nextURL := fmt.Sprintf("%s://%s/v2/%s/tags/list?n=100", config.Scheme, config.URL, repo)

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create tags request: %w", err)
		}

		if config.Username != "" && config.Password != "" {
			req.SetBasicAuth(config.Username, config.Password)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to query tags: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("tags request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var tagsResp TagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode tags response: %w", err)
		}
		resp.Body.Close()

		allTags = append(allTags, tagsResp.Tags...)

		// Check for next page in Link header
		nextURL = getNextPageURL(resp.Header.Get("Link"), config.Scheme, config.URL)
	}

	return allTags, nil
}

// getNextPageURL parses the Link header to extract the next page URL
// Link header format: </v2/_catalog?n=100&last=repo99>; rel="next"
func getNextPageURL(linkHeader, scheme string, baseURL string) string {
	if linkHeader == "" {
		return ""
	}

	// Parse Link header
	parts := strings.Split(linkHeader, ";")
	if len(parts) < 2 {
		return ""
	}

	// Extract URL from <...>
	urlPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
		return ""
	}
	urlPart = strings.Trim(urlPart, "<>")

	// Check if it's a relative or absolute URL
	if strings.HasPrefix(urlPart, "http://") || strings.HasPrefix(urlPart, "https://") {
		return urlPart
	}

	// If relative, construct full URL
	parsedBase, err := url.Parse(fmt.Sprintf("%s://%s", scheme, baseURL))
	if err != nil {
		return ""
	}

	// Combine base URL with relative path
	if strings.HasPrefix(urlPart, "/") {
		return fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, urlPart)
	}

	return fmt.Sprintf("%s%s", baseURL, urlPart)
}

func filter(images []imageInfo, re *regexp.Regexp) []imageInfo {
	filteredImages := []imageInfo{}
	for _, image := range images {
		fullImage := fmt.Sprintf("%s:%s\n", image.Repository, image.Tags[0])
		if re.MatchString(fullImage) {
			filteredImages = append(filteredImages, image)
		}
	}
	return filteredImages
}

// getImages gets all the images directly from the upstream that the ociregistry server will
// be pulling from. This comprises the test set. Scheme is hard-coded to HTTP for now.
func getImages(registryUrl string, re *regexp.Regexp) ([]imageInfo, error) {
	config := registryConfig{
		URL:      registryUrl,
		Scheme:   "http",
		Username: "",
		Password: "",
	}

	fmt.Println("Listing images from the upstream")
	images, err := listAllImages(config)
	if err != nil {
		return []imageInfo{}, err
	}
	fmt.Printf("Got %d images\n", len(images))

	if re != nil {
		images = filter(images, re)
		fmt.Printf("Have %d images after applying filter\n", len(images))
	}

	return images, nil
}
