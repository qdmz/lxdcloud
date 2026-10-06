package auth

type APILevel int

const (
	LevelSystem    APILevel = iota
	LevelAdmin
	LevelUser
	LevelContainer
)

func (l APILevel) String() string {
	switch l {
	case LevelSystem:
		return "system"
	case LevelAdmin:
		return "admin"
	case LevelUser:
		return "user"
	case LevelContainer:
		return "container"
	default:
		return "unknown"
	}
}

func (l APILevel) CanAccess(required APILevel) bool {
	return l <= required
}

