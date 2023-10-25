package group

func (g *Group) Service() IService {
	return g.groupService
}
