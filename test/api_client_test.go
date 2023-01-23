package test

import (
	"cmk_getter/utils"
	"testing"
)

func TestGetCmkVersion(t *testing.T) {
	cmkResponse := `{
  "site": "mysite",
  "group": "",
  "rest_api": {
    "revision": "0"
  },
  "versions": {
    "apache": [
      2,
      4,
      38
    ],
    "checkmk": "2.1.0p14.cre",
    "python": "3.9.10 (main, Jul  6 2022, 22:25:16) \n[GCC 11.2.0]",
    "mod_wsgi": [
      4,
      9,
      0
    ],
    "wsgi": [
      1,
      0
    ]
  },
  "edition": "cre",
  "demo": false
}`
	expectedVersion := "2.1.0p14.cre"
	// Convert cmkResponse to []byte
	cmkResponseBytes := []byte(cmkResponse)
	versionResp, err := utils.GetCmkVersion(cmkResponseBytes)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if versionResp.Versions.Checkmk != expectedVersion {
		t.Errorf("Expected %s, got %s", expectedVersion, versionResp.Versions.Checkmk)
	}
}
