package user

import (
	common2 "github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/model"
)

func (u *User) Db() *DB {
	return u.db
}

func (u *User) FriendDB() *friendDB {
	return u.friendDB
}

func (u *User) OnlineService() *OnlineService {
	return u.onlineService
}

func (u *User) CommonService() common2.IService {
	return u.commonService
}

func (u *User) UserService() IService {
	return u.userService
}

func NewChannelRespWithUserDetailResp(user *UserDetailResp) *model.ChannelResp {
	return newChannelRespWithUserDetailResp(user)
}
