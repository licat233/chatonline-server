package group

func (g *Group) Service() IService {
	return g.groupService
}

func (g *Group) GroupDB() *DB {
	return g.db
}
