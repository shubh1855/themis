package tools

type PermissionMode int

const (
	Deny PermissionMode = iota
	AllowOnce
	AllowAlways
)

type PermissionRequest struct {
	Tool string
	Path string
}

type PermissionManager struct {
	allowAll bool
}

func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

func (p *PermissionManager) IsGloballyAllowed() bool {
	return p.allowAll
}

func (p *PermissionManager) Resolve(choice PermissionMode) {
	if choice == AllowAlways {
		p.allowAll = true
	}
}

func (p *PermissionManager) NeedsPrompt() bool {
	return !p.allowAll
}
