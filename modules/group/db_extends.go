package group

// QueryGroupByNo 查询某个群,通过groupNo
func (d *DB) QueryGroupWithGroupNo(groupNo string) (*Model, error) {
	var model *Model
	_, err := d.session.Select("*").From("`group`").Where("group_no=?", groupNo).Load(&model)
	return model, err
}

// 查询指定群内的所有成员
func (d *DB) QueryMembersWithGroupNo(groupNo string) ([]*MemberModel, error) {
	var memberModels []*MemberModel
	_, err := d.session.Select("*").From("group_member").Where("group_no=? and is_deleted=0", groupNo).Load(&memberModels)
	return memberModels, err
}

// 修改群成员备注
func (d *DB) UpdateMemberRemark(id int64, remark string) error {
	_, err := d.session.Update("group_member").SetMap(map[string]interface{}{
		"remark": remark,
	}).Where("id=?", id).Exec()
	return err
}

// 查找可用的客服
// visitorUID是需要排除掉的uid
func (d *DB) QueryKefuMembers(groupNo string, visitorUID string) ([]*MemberModel, error) {
	var memberModels []*MemberModel
	var err error
	if visitorUID != "" {
		_, err = d.session.Select("*").From("group_member").Where("group_no=? and uid<>? and is_deleted=0 and role=0 and robot=0 and status=1", groupNo, visitorUID).Load(&memberModels)
	} else {
		_, err = d.session.Select("*").From("group_member").Where("group_no=? and is_deleted=0 and role=0 and robot=0 and status=1", groupNo).Load(&memberModels)
	}

	return memberModels, err
}

// 查找可用的客服uid
// visitorUID是需要排除掉的uid
func (d *DB) QueryKefuUidMembers(groupNo string, visitorUID string) ([]string, error) {
	var uids []string
	var err error
	if visitorUID != "" {
		_, err = d.session.Select("uid").From("group_member").Where("group_no=? and uid=? and is_deleted=0 and role=0 and robot=0 and status=1", groupNo, visitorUID).Load(&uids)
	} else {
		_, err = d.session.Select("uid").From("group_member").Where("group_no=? and is_deleted=0 and role=0 and robot=0 and status=1", groupNo).Load(&uids)
	}

	return uids, err
}

// 查找某个user是否在群内
func (d *DB) ExistMemberInGroup(uid string, groupNo string) (bool, error) {
	var count int
	_, err := d.session.Select("COUNT(*)").From("group_member").Where("group_no=? and uid=?", groupNo, uid).Limit(1).Load(&count)
	return count != 0, err
}

// 修改群名称
func (d *DB) UpdateGroupName(groupNo string, name string) error {
	_, err := d.session.Update("group").Set("name", name).Where("group_no=?", groupNo).Exec()
	return err
}
