package plugin

import (
	"context"
	"github.com/gin-gonic/gin"
)

type Plugin interface {
	Name() string
	Version() string
	Description() string
	
	Init() error
	Start() error
	Stop() error
	
	RegisterRoutes(r *gin.Engine)
	RegisterHooks(h *HookManager)
}

type HookPoint string

const (
	HookBeforeContainerCreate HookPoint = "before_container_create"
	HookAfterContainerCreate  HookPoint = "after_container_create"
	HookBeforeContainerStart  HookPoint = "before_container_start"
	HookAfterContainerStart   HookPoint = "after_container_start"
	HookBeforeContainerStop   HookPoint = "before_container_stop"
	HookAfterContainerStop    HookPoint = "after_container_stop"
	HookBeforeContainerDelete HookPoint = "before_container_delete"
	HookAfterContainerDelete  HookPoint = "after_container_delete"
	HookTrafficOverLimit      HookPoint = "traffic_over_limit"
	HookIPv4Allocated         HookPoint = "ipv4_allocated"
	HookIPv4Released          HookPoint = "ipv4_released"
)

type HookFunc func(ctx context.Context, data map[string]interface{}) error

type HookManager struct {
	hooks map[HookPoint][]HookFunc
}

func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make(map[HookPoint][]HookFunc),
	}
}

func (h *HookManager) Register(point HookPoint, fn HookFunc) {
	h.hooks[point] = append(h.hooks[point], fn)
}

func (h *HookManager) Trigger(point HookPoint, ctx context.Context, data map[string]interface{}) error {
	for _, fn := range h.hooks[point] {
		if err := fn(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookManager) TriggerAsync(point HookPoint, ctx context.Context, data map[string]interface{}) {
	for _, fn := range h.hooks[point] {
		go fn(ctx, data)
	}
}

