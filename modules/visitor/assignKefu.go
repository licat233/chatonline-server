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
	//如果是老访客
	if v.isOldVisitor {
		//查找群里在线的客服
		kefuUids, err := v.getOnlineUsersFromGroup(v.channelID, v.visitor.UID)
		if err != nil {
			v.Error("查询群成员错误", zap.Error(err))
			return nil, err
		}
		if len(kefuUids) > 0 {
			//群里存在在线的客服，不用分配，随便返回一个即可
			for _, kefuUid := range kefuUids {
				kefu, err := v.user.Db().QueryByUID(kefuUid)
				if err != nil {
					v.Error("查询用户信息错误", zap.Error(err))
					return nil, err
				}
				if kefu == nil {
					//不存在
					continue
				}
				if kefu.Status == 0 {
					continue
				}
				return kefu, nil
			}
			//到了这一步，表示没有状态可用的在线客服
			//移除他们
			err = v.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
				OperatorUid:  v.visitor.UID,
				OperatorName: OperatorName,
				Members:      kefuUids,
				GroupNo:      v.channelID,
			})
			if err != nil {
				v.Error("移除群成员错误", zap.Error(err))
				return nil, err
			}
		}
	}
	//如果还是没有，那就从项目群里取一个在线的客服

	//从项目群组中分配一个客服出来作为该游客的专属客服
	//查找该群里有多少客服
	members, err := v.groupDB.QueryKefuMembers(v.currentProjectGroupModel.GroupNo, v.visitor.UID)
	if err != nil {
		v.Error("查询群成员信息错误", zap.Error(err))
		return nil, err
	}
	if len(members) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}
	//取出uid
	var allUids []string
	for _, gmember := range members {
		allUids = append(allUids, gmember.UID)
	}
	//查找看有无在线人数
	onlineUids, err := v.getOnlineUsers(allUids)
	if err != nil {
		return nil, err
	}
	//如果没有在线的uid
	if len(onlineUids) == 0 {
		//则默认取所有离线客服
		onlineUids = allUids
	}

	//取出所有的user信息
	kefuList, err := v.user.Db().QueryKefuByUIDs(allUids)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return nil, err
	}
	//没有可用客服
	if len(kefuList) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}

	var kefuMembers []*group.MemberModel
	//筛选出可用会员
	for _, kf := range kefuList {
		for _, member := range members {
			if kf.UID == member.UID {
				kefuMembers = append(kefuMembers, member)
			}
		}
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
	for _, member := range kefuMembers {
		// 有标记
		if strings.HasSuffix(member.Remark, ".") {
			markMembers = append(markMembers, member)
		} else {
			noMarkMembers = append(noMarkMembers, member)
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
	//找出对应的user
	uid := randomMember.UID
	for _, user := range kefuList {
		if user.UID == uid {
			if err := addRemark(randomMember); err != nil {
				return nil, err
			}
			return user, nil
		}
	}

	//找不到？
	v.Error("无对应的kefu用户")
	return nil, errors.New("system error! group user does not exist")
}

// 查找群里在线的客服
func (v *Visitor) getOnlineUsersFromGroup(groupNo string, visitorUID string) ([]string, error) {
	//查找聊天群里所有的客服
	kefuUids, err := v.groupDB.QueryKefuUidMembers(groupNo, visitorUID)
	if err != nil {
		v.Error("查询群成员错误", zap.Error(err))
		return nil, err
	}
	//查找在线的客服
	onlineUids, err := v.getOnlineUsers(kefuUids)
	if err != nil {
		return nil, err
	}
	return onlineUids, nil
}

// 查询在线的users， 10分钟内离线的也算是在线客服
func (v *Visitor) getOnlineUsers(uids []string) ([]string, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	all, err := v.user.OnlineDB().QueryOnlineWithUIDs(uids)
	if err != nil {
		v.Error("查询在线用户错误", zap.Error(err))
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	//先查看是否存在在线的客服
	var onlineUserUids []string
	for _, onlineUser := range all {
		if onlineUser.Online == 1 {
			onlineUserUids = append(onlineUserUids, onlineUser.UID)
		}
	}
	//如果存在，直接返回
	if len(onlineUserUids) != 0 {
		return onlineUserUids, nil
	}
	//如果不存在，查看是否存在10分钟内离线的客服
	var list []string
	for _, onlineUser := range all {
		if onlineUser.IsOnline() {
			list = append(list, onlineUser.UID)
		}
	}
	return list, nil
}

func (v *Visitor) kefuIsOnline(uid string) (bool, error) {
	//离线小于10分钟，不算做离线
	onlineModel, err := v.user.OnlineDB().QueryOnlineWithUID(uid)
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
