package visitor

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"go.uber.org/zap"
)

// 根据当前游客情况，分配一个客服出来
func (v *Visitor) assignKefu() (*user.Model, error) {
	if v.channelID != "" {
		//查找聊天群里所有的客服
		members, err := v.groupDB.QueryMembersWithGroupNo(v.channelID)
		if err != nil {
			v.Error("查询群成员错误", zap.Error(err))
			return nil, err
		}
		var uidList []string
		for _, gmember := range members {
			uidList = append(uidList, gmember.UID)
		}
		//查找在线的客服
		onlineUids, err := v.getOnlineUsers(uidList)
		if err != nil {
			return nil, err
		}
		if len(onlineUids) > 0 {
			//群里存在在线的客服，不用分配，随便返回一个即可
			kefuUid := onlineUids[0]
			kefu, err := v.user.Db().QueryByUID(kefuUid)
			if err != nil {
				v.Error("查询用户信息错误", zap.Error(err))
				return nil, err
			}
			return kefu, nil
		}
		//如果没有实时在线的客服，那就看是否有10分钟内离线的客服
		for _, uid := range uidList {
			exist, err := v.kefuIsOnline(uid)
			if err != nil {
				return nil, err
			}
			if exist {
				kefuUid := uid
				kefu, err := v.user.Db().QueryByUID(kefuUid)
				if err != nil {
					v.Error("查询用户信息错误", zap.Error(err))
					return nil, err
				}
				return kefu, nil
			}
		}
	}
	//如果还是没有，那就从项目群里取一个在线的客服

	//从项目群组中分配一个客服出来作为该游客的专属客服
	//查找该群里有多少客服
	members, err := v.groupDB.QueryKefuMembers(v.currentProjectGroupModel.GroupNo)
	if err != nil {
		v.Error("查询群成员信息错误", zap.Error(err))
		return nil, err
	}
	if len(members) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}
	//再过滤一遍，排除掉系统账号
	var uids []string
	for _, gmember := range members {
		uids = append(uids, gmember.UID)
	}
	userList, err := v.user.Db().QueryByUIDs(uids)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return nil, err
	}
	var users []*user.Model
	uids = []string{}
	for _, user := range userList {
		if user.Category == "system" || user.Category == "visitor" {
			continue
		}
		uids = append(uids, user.UID)
		users = append(users, user)
	}
	//logic: 查询在线的用户
	onlines, err := v.user.OnlineService().QueryOnlineWithUIDs(uids)
	if err != nil {
		v.Error("查询在线用户错误", zap.Error(err))
		return nil, err
	}

	if len(onlines) != 0 {
		//有在線人數，分配给在线的客服，筛选出在线客服uid
		uids = []string{}
		for _, online := range onlines {
			uids = append(uids, online.UID)
		}
	}
	//没有在线人数，则默认分配给所有离线客服
	if len(uids) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}

	//确保每个客服在当日内分配到的游客数一样多：采用标记分配法
	// 	具体步骤如下：
	// 1.给每个人设置一个初始状态为未标记。这里给群成员备足标记📌为 xxxx.
	// 2.开始分配苹果时，从未标记的人中随机选择一个人进行分配，并将其标记为已分配过苹果。
	// 3.重复第2步，直到所有人都被标记为已分配过苹果。
	// 4.当所有人都被标记后，表示完成了一次轮回分配。清除所有人的标记，重复第2步开始新的轮回分配。

	//查找出未标记的群成员
	var noMarkMembers []*group.MemberModel
	var markMembers []*group.MemberModel
	for _, uid := range uids {
		for _, member := range members {
			if uid == member.UID {
				// 有标记
				if strings.HasSuffix(member.Remark, ".") {
					markMembers = append(markMembers, member)
				} else {
					noMarkMembers = append(noMarkMembers, member)
				}
			}
		}
	}

	//检查是否存在未标记的群成员
	if len(noMarkMembers) == 0 {
		if len(markMembers) == 0 {
			//特殊情况
			v.Error("member does not exist")
			return nil, errors.New("member does not exist")
		}
		//如果不存在未标记的群成员，则开启新的轮回分配
		//清除所有的标记
		for _, member := range markMembers {
			for strings.HasSuffix(member.Remark, ".") {
				member.Remark = strings.TrimSuffix(member.Remark, ".")
			}
			if err := v.groupDB.UpdateMemberRemark(member.Id, member.Remark); err != nil {
				v.Error("更新群成员备注失败", zap.Error(err))
				return nil, err
			}
		}
		noMarkMembers = markMembers
	}

	var addRemark = func(m *group.MemberModel) error {
		if strings.HasSuffix(m.Remark, ".") {
			return nil
		}
		m.Remark = fmt.Sprintf("%s.", m.Remark)
		if err := v.groupDB.UpdateMemberRemark(m.Id, m.Remark); err != nil {
			v.Error("更新群成员备注失败", zap.Error(err))
			return fmt.Errorf("更新群成员备注失败: %s", err)
		}
		return nil
	}

	//从未标记的群成员中随机挑选出一个
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomIndex := random.Intn(len(noMarkMembers))
	randomMember := noMarkMembers[randomIndex]
	uid := randomMember.UID
	for _, user := range users {
		if user.UID == uid {
			//如果已经在users中了，没必要再继续查询，直接返回即可
			if err := addRemark(randomMember); err != nil {
				return nil, err
			}
			return user, nil
		}
	}

	user, err := v.user.Db().QueryByUID(uid)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return nil, err
	}
	if user == nil {
		v.Error("查询用户不存在")
		return nil, errors.New("system error! user does not exist")
	}
	if err := addRemark(randomMember); err != nil {
		return nil, err
	}
	return user, nil
}

func (v *Visitor) getOnlineUsers(uids []string) ([]string, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	onlines, err := v.user.OnlineService().QueryOnlineWithUIDs(uids)
	if err != nil {
		v.Error("查询在线用户错误", zap.Error(err))
		return nil, err
	}
	res := []string{}
	for _, online := range onlines {
		res = append(res, online.UID)
	}
	return res, nil
}

func (v *Visitor) kefuIsOnline(uid string) (bool, error) {
	//离线小于10分钟，不算做离线
	onlineModel, err := v.user.OnlineService().QueryOnlineWithUID(uid)
	if err != nil {
		v.Error("查询在线信息错误", zap.Error(err))
		return false, err
	}
	if onlineModel == nil {
		return false, errors.New("kefu online info does not exist")
	}
	if onlineModel.Online == 1 {
		//如果专属客服在线，则返回原客服
		return true, nil
	} else {
		// 如果专属客服不在线，且离线超过10分钟
		t := time.Unix(int64(onlineModel.LastOffline), 0)
		//离线时间小于10分钟
		if time.Since(t).Minutes() < 10 {
			//不用换客服
			return true, nil
		}
		// 则根据 reset 决定是否分配新的客服
		if !v.req.RestKefu {
			// 如果不重新分配，则返回原客服
			return true, nil
		}
	}
	return false, nil
}

// func (v *Visitor) getOnlineMemberWithGroupNo(groupNo string) ([]string, error) {
// 	members, err := v.groupDB.QueryKefuMembers(groupNo)
// 	if err != nil {
// 		v.Error("查询群成员信息错误", zap.Error(err))
// 		return nil, err
// 	}
// 	n := len(members)
// 	if n == 0 {
// 		return nil, nil
// 	}
// 	var uids []string
// 	for _, gmember := range members {
// 		uids = append(uids, gmember.UID)
// 	}
// 	uids, err = v.getOnlineUsers(uids)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var res []string
// 	for _, uid := range uids {
// 		for _, gmember := range members {
// 			if uid == gmember.UID {
// 				res = append(res, uid)
// 			}
// 		}
// 	}
// 	return res, nil
// }
