package service

import (
	"errors"
	"testing"

	"messagefeed/internal/domain"
)

func TestXSubscriptionRoutes(t *testing.T) {
	address, route, _, err := xSubscription("")
	if err != nil || address != "https://x.com/home" || route != "/twitter/home_latest" {
		t.Fatalf("following subscription = %q, %q, %v", address, route, err)
	}
	address, route, _, err = xSubscription(" @ChromeDevTools ")
	if err != nil || address != "https://x.com/chromedevtools" || route != "/twitter/user/chromedevtools" {
		t.Fatalf("account subscription = %q, %q, %v", address, route, err)
	}
	for _, input := range []string{"@", "@home", "../home_latest", "name?format=json", "中文"} {
		if _, _, _, err := xSubscription(input); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("xSubscription(%q) = %v", input, err)
		}
	}
}

func TestXConnectorRequiresDistinctBrowserHosts(t *testing.T) {
	if _, err := NewXConnector(`{"1":"http://x-browser-1:5800","2":"http://X-browser-1.:5800"}`); err == nil {
		t.Fatal("bindings sharing a browser must be rejected")
	}
}
