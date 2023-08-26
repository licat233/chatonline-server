package group

// QueryGroupByNo 查询某个群,通过groupNo
// func (d *DB) QueryGroupWithGroupNo(groupNo string) (*Model, error) {
// 	var model *Model
// 	_, err := d.session.Select("*").From("group").Where("`group_no`=?", groupNo).Load(&model)
// 	return model, err
// }

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
