package bootstrap

import (
	"fmt"
	"messagefeed/internal/config"
)

type RolePlan struct {
	API                  bool
	SourceWorker         bool
	NotificationWorker   bool
	AgentSchedulerWorker bool
	EmbeddingWorker      bool
	Migrate              bool
}

func PlanForRole(role config.AppRole) (RolePlan, error) {
	switch role {
	case config.AppRoleAll:
		return RolePlan{API: true, SourceWorker: true, NotificationWorker: true, AgentSchedulerWorker: true, EmbeddingWorker: true}, nil
	case config.AppRoleAPI:
		return RolePlan{API: true}, nil
	case config.AppRoleSourceWorker:
		return RolePlan{SourceWorker: true}, nil
	case config.AppRoleNotificationWorker:
		return RolePlan{NotificationWorker: true}, nil
	case config.AppRoleAgentSchedulerWorker:
		return RolePlan{AgentSchedulerWorker: true}, nil
	case config.AppRoleEmbeddingWorker:
		return RolePlan{EmbeddingWorker: true}, nil
	case config.AppRoleMigrate:
		return RolePlan{Migrate: true}, nil
	default:
		return RolePlan{}, fmt.Errorf("unsupported application role %q", role)
	}
}

func (plan RolePlan) HasWorkers() bool {
	return plan.SourceWorker || plan.NotificationWorker || plan.AgentSchedulerWorker || plan.EmbeddingWorker
}
