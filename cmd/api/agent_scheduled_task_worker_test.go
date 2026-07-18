package main

import (
	"messagefeed/internal/bootstrap"
	"messagefeed/internal/config"
	"testing"
)

func TestEntrypointRolePlansAreExplicit(t *testing.T) {
	tests := []struct {
		role       config.AppRole
		wantAPI    bool
		wantWorker bool
	}{
		{role: config.AppRoleAPI, wantAPI: true},
		{role: config.AppRoleSourceWorker, wantWorker: true},
		{role: config.AppRoleNotificationWorker, wantWorker: true},
		{role: config.AppRoleAgentSchedulerWorker, wantWorker: true},
		{role: config.AppRoleEmbeddingWorker, wantWorker: true},
		{role: config.AppRoleMigrate},
	}
	for _, test := range tests {
		plan, err := bootstrap.PlanForRole(test.role)
		if err != nil {
			t.Fatalf("PlanForRole(%q) error = %v", test.role, err)
		}
		if plan.API != test.wantAPI {
			t.Errorf("PlanForRole(%q).API = %t, want %t", test.role, plan.API, test.wantAPI)
		}
		if plan.HasWorkers() != test.wantWorker {
			t.Errorf("PlanForRole(%q).HasWorkers() = %t, want %t", test.role, plan.HasWorkers(), test.wantWorker)
		}
	}
}
