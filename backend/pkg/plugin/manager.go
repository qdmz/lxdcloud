package plugin

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"lxdapi/pkg/logger"
	"sync"
)

var (
	GlobalManager *Manager
	once          sync.Once
)

type Manager struct {
	plugins     map[string]Plugin
	hookManager *HookManager
	mu          sync.RWMutex
}

func InitManager() *Manager {
	once.Do(func() {
		GlobalManager = &Manager{
			plugins:     make(map[string]Plugin),
			hookManager: NewHookManager(),
		}
	})
	return GlobalManager
}

func GetManager() *Manager {
	if GlobalManager == nil {
		return InitManager()
	}
	return GlobalManager
}

func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("插件已存在: %s", name)
	}

	if err := p.Init(); err != nil {
		return fmt.Errorf("插件初始化失败 %s: %v", name, err)
	}

	p.RegisterHooks(m.hookManager)
	m.plugins[name] = p

	logger.OK("插件注册成功: %s v%s - %s", name, p.Version(), p.Description())
	return nil
}

func (m *Manager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, p := range m.plugins {
		if err := p.Start(); err != nil {
			logger.Error("插件启动失败 %s: %v", name, err)
			return err
		}
		logger.OK("插件启动: %s", name)
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, p := range m.plugins {
		if err := p.Stop(); err != nil {
			logger.Error("插件停止失败 %s: %v", name, err)
		} else {
			logger.OK("插件停止: %s", name)
		}
	}
}

func (m *Manager) RegisterRoutes(r *gin.Engine) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.plugins {
		p.RegisterRoutes(r)
	}
}

func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.plugins[name]
	return p, exists
}

func (m *Manager) ListPlugins() []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]map[string]string, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, map[string]string{
			"name":        p.Name(),
			"version":     p.Version(),
			"description": p.Description(),
		})
	}
	return list
}

func (m *Manager) GetHookManager() *HookManager {
	return m.hookManager
}

