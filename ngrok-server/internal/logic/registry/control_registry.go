package registry

import (
	"fmt"
	"ngrok-common/log"
	"ngrok-server/internal/service"
	"sync"
)

type localControlRegistry struct {
	controls map[string]*service.Control
	log.Logger
	sync.RWMutex
}

func NewControlRegistry() service.ControlRegistryService {
	return &localControlRegistry{
		controls: make(map[string]*service.Control),
		Logger:   log.NewPrefixLogger("registry", "ctl"),
	}
}

// 初始化服务
func init() {
	service.RegisterControlRegistry(NewControlRegistry())
}

func (r *localControlRegistry) Get(clientId string) *service.Control {
	r.RLock()
	defer r.RUnlock()
	return r.controls[clientId]
}

func (r *localControlRegistry) Add(clientId string, ctl *service.Control) (oldCtl *service.Control) {
	r.Lock()
	defer r.Unlock()

	if oldCtl = r.controls[clientId]; oldCtl != nil {
		oldCtl.Replaced(ctl)
	}

	r.controls[clientId] = ctl
	r.Info("Registered control with id %s", clientId)
	return
}

func (r *localControlRegistry) Del(clientId string) error {
	r.Lock()
	defer r.Unlock()

	if r.controls[clientId] == nil {
		return fmt.Errorf("no control found for client id: %s", clientId)
	} else {
		r.Info("Remove control registry id %s", clientId)
		delete(r.controls, clientId)
		return nil
	}
}
