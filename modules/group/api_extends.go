package group

import (
	"errors"
	"fmt"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/base/event"
	chservice "github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/channel/service"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/register"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkevent"
	"go.uber.org/zap"
)

type VisitorGroupCreateReq struct {
	CreateUid string   `json:"create_uid"` // 创建者uid
	Name      string   `json:"name"`       // 群名
	Members   []string `json:"members"`    // 成员uid
	GroupNo   string   `json:"group_no"`   // 群编号，格式为 visitorUid-projectUid 的组合
	Notice    string   `json:"notice"`     // 群公告
}

func (g *Group) VisitorGroupCreate(req *VisitorGroupCreateReq) (*Model, error) {
	creator := req.CreateUid
	creatorName := req.Name
	creatorUser, err := g.userDB.QueryByUID(req.CreateUid)
	if err != nil {
		g.Error("查询创建者信息失败！", zap.Error(err))
		return nil, errors.New("查询创建者信息失败！")
	}
	if creatorUser == nil {
		g.Error("创建者不存在！", zap.String("creator", creator))
		return nil, errors.New("创建者不存在！")
	}

	req.Members = util.RemoveRepeatedElement(append(req.Members, creator)) // 将创建者也加入成员内

	// 查询成员用户信息
	memberUserModels, err := g.userDB.QueryByUIDs(req.Members)
	if err != nil {
		g.Error("查询成员用户信息失败！", zap.Error(err), zap.Strings("members", req.Members))
		return nil, errors.New("查询成员用户信息失败！")
	}
	if memberUserModels == nil {
		return nil, errors.New("成员用户信息不存在！")
	}
	groupName := req.Name

	groupNo := req.GroupNo

	version := g.ctx.GenSeq(common.GroupSeqKey)
	channelServiceObj := register.GetService(ChannelServiceName)
	var channelService chservice.IService
	if channelServiceObj != nil {
		channelService = channelServiceObj.(chservice.IService)
	}
	if channelService != nil {
		if creatorUser != nil && creatorUser.MsgExpireSecond > 0 {
			err = channelService.CreateOrUpdateMsgAutoDelete(groupNo, common.ChannelTypeGroup.Uint8(), creatorUser.MsgExpireSecond)
			if err != nil {
				g.Warn("更新消息自动删除失败！", zap.Error(err))
			}
		}
	}

	tx, err := g.ctx.DB().Begin()
	util.CheckErr(err)
	defer func() {
		if err := recover(); err != nil {
			tx.RollbackUnlessCommitted()
			panic(err)
		}
	}()

	err = g.db.InsertTx(&Model{
		GroupNo:             groupNo,
		Name:                groupName,
		Creator:             creator,
		Status:              GroupStatusNormal,
		Version:             version,
		AllowViewHistoryMsg: int(common.GroupAllowViewHistoryMsgEnabled),
		Notice:              req.Notice,
	}, tx)
	if err != nil {
		g.Error("添加群失败！", zap.Error(err))
		tx.RollbackUnlessCommitted()
		return nil, errors.New("添加群失败！")
	}
	realMemberUids := make([]string, 0) // 真实成员uid集合
	userBaseVos := make([]*config.UserBaseVo, 0)
	// 注销用户
	destroyUserBaseVos := make([]*config.UserBaseVo, 0)
	for _, memberUser := range memberUserModels {
		if memberUser.IsDestroy == 1 {
			destroyUserBaseVos = append(destroyUserBaseVos, &config.UserBaseVo{
				UID:  memberUser.UID,
				Name: memberUser.Name,
			})
			continue
		}
		memberVersion := g.ctx.GenSeq(common.GroupMemberSeqKey)
		realMemberUids = append(realMemberUids, memberUser.UID)
		var role = MemberRoleCommon
		if memberUser.UID == creator {
			role = MemberRoleCreator
		}
		err = g.db.InsertMemberTx(&MemberModel{
			GroupNo:   groupNo,
			UID:       memberUser.UID,
			Role:      role,
			Version:   memberVersion,
			InviteUID: creator,
			Robot:     memberUser.Robot,
			Status:    int(common.GroupMemberStatusNormal),
			Vercode:   fmt.Sprintf("%s@%d", util.GenerUUID(), common.GroupMember),
		}, tx)
		if err != nil {
			tx.RollbackUnlessCommitted()
			g.Error("添加成员失败！", zap.Error(err), zap.String("memberUid", memberUser.UID))
			return nil, errors.New("添加成员失败！")
		}
		userBaseVos = append(userBaseVos, &config.UserBaseVo{UID: memberUser.UID, Name: memberUser.Name})
	}
	if len(realMemberUids) <= 0 {
		tx.RollbackUnlessCommitted()
		g.Error("群成员不能为空！")
		return nil, errors.New("群成员不能为空！")
	}
	// 发布群创建事件
	createGroupEventID, err := g.ctx.EventBegin(&wkevent.Data{
		Event: event.GroupCreate,
		Type:  wkevent.Message,
		Data: &config.MsgGroupCreateReq{
			GroupNo:     groupNo,
			Creator:     creator,
			CreatorName: creatorName,
			Members:     userBaseVos,
			Version:     version,
		},
	}, tx)
	if err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("开启事件失败！", zap.Error(err))
		return nil, errors.New("开启事件失败！")
	}
	var unableAddDestroyAccount int64 = 0
	if len(destroyUserBaseVos) > 0 {
		// 发布无法添加到群聊用户
		unableAddDestroyAccount, err = g.ctx.EventBegin(&wkevent.Data{
			Event: event.GroupUnableAddDestroyAccount,
			Type:  wkevent.Message,
			Data: &config.MsgGroupCreateReq{
				GroupNo:     groupNo,
				Creator:     creator,
				CreatorName: creatorName,
				Members:     destroyUserBaseVos,
				Version:     version,
			},
		}, tx)
		if err != nil {
			tx.RollbackUnlessCommitted()
			g.Error("开启无法添加到群聊事件失败！", zap.Error(err))
			return nil, errors.New("开启无法添加到群聊事件失败！")
		}
	}
	groupAvatarEventID, err := g.ctx.EventBegin(&wkevent.Data{
		Event: event.GroupAvatarUpdate,
		Type:  wkevent.CMD,
		Data: &config.CMDGroupAvatarUpdateReq{
			GroupNo: groupNo,
			Members: realMemberUids,
		},
	}, tx)
	if err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("开启群成员头像更新事件失败！", zap.Error(err))
		return nil, errors.New("开启群成员头像更新事件失败！")
	}

	// 创建IM频道
	err = g.ctx.IMCreateOrUpdateChannel(&config.ChannelCreateReq{
		ChannelID:   groupNo,
		ChannelType: common.ChannelTypeGroup.Uint8(),
		Subscribers: realMemberUids,
	})
	if err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("创建IM频道失败！", zap.Error(err))
		return nil, errors.New("创建IM频道失败！")
	}
	if err := tx.Commit(); err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("提交事务失败！", zap.Error(err))
		return nil, errors.New("提交事务失败！")
	}
	g.ctx.EventCommit(createGroupEventID)
	g.ctx.EventCommit(groupAvatarEventID)
	if unableAddDestroyAccount != 0 {
		g.ctx.EventCommit(unableAddDestroyAccount)
	}
	groupModel, err := g.db.QueryWithGroupNo(groupNo)
	if err != nil {
		g.Error("查询群信息失败！", zap.Error(err))
		return nil, errors.New("查询群信息失败！")
	}
	return groupModel, nil
}

type VisitorMemberAddReq struct {
	OperatorUid  string
	OperatorName string
	Members      []string
	GroupNo      string
}

// 添加成员
func (g *Group) VisitorMemberAdd(req *VisitorMemberAddReq) error {
	operator := req.OperatorUid
	operatorName := req.OperatorName
	groupNo := req.GroupNo
	/**
	判断群是否存在
	**/
	group, err := g.db.QueryWithGroupNo(groupNo)
	if err != nil {
		g.Error("查询群信息失败！", zap.Error(err), zap.String("groupNo", groupNo))
		return errors.New("查询群信息失败！")
	}
	if group == nil {
		return errors.New("群不存在！")
	}
	/**
	判断群是否开启了邀请模式 如果开启了 再判断邀请的人是否是群主或管理员 如果不是则不允许直接添加群成员
	**/
	if group.Invite == 1 {
		creatorOrManager, err := g.db.QueryIsGroupManagerOrCreator(groupNo, operator)
		if err != nil {
			g.Error("查询是否是创建者和管理者失败！", zap.Error(err))
			return errors.New("查询是否是创建者和管理者失败！")
		}
		if !creatorOrManager {
			return errors.New("群开启了邀请模式，不能添加群成员！")
		}
	}

	err = g.addMembers(req.Members, groupNo, operator, operatorName)
	if err != nil {
		return err
	}

	memberCount, err := g.db.QueryMemberCount(groupNo)
	if err != nil {
		g.Error("查询群成员数量失败！", zap.Error(err))
		return errors.New("查询群成员数量失败！")
	}

	// 普通群自动升级
	if memberCount >= int64(g.ctx.GetConfig().GroupUpgradeWhenMemberCount) && group.GroupType == int(GroupTypeCommon) {
		var ban = 0
		if group.Status == GroupStatusDisabled {
			ban = 1
		}
		err = g.ctx.IMCreateOrUpdateChannel(&config.ChannelCreateReq{
			ChannelID:   groupNo,
			ChannelType: common.ChannelTypeGroup.Uint8(),
			Ban:         ban,
			Large:       1,
		})
		if err != nil {
			g.Error("更新频道信息失败！", zap.Error(err))
			return errors.New("更新频道信息失败！")
		}

		err = g.db.UpdateGroupType(groupNo, GroupTypeSuper)
		if err != nil {
			g.Error("修改群为超级群失败！", zap.Error(err), zap.String("groupNo", groupNo))
			return errors.New("修改群为超级群失败！")
		}
		// 发送群升级通知
		// err = g.ctx.SendGroupUpgrade(groupNo)
		// if err != nil {
		// 	g.Warn("发送群升级通知失败！", zap.Error(err))
		// }
	}
	return nil
}

type VisitorMemberRemoveReq struct {
	OperatorUid  string
	OperatorName string
	Members      []string
	GroupNo      string
}

// 移除群成员
func (g *Group) VisitorMemberRemove(req *VisitorMemberRemoveReq) error {
	operator := req.OperatorUid
	operatorName := req.OperatorName
	groupNo := req.GroupNo
	req.Members = util.RemoveRepeatedElement(req.Members)

	// 判断群是否存在
	group, err := g.db.QueryWithGroupNo(groupNo)
	if err != nil {
		g.Error("查询群信息失败！", zap.Error(err), zap.String("groupNo", groupNo))
		return errors.New("查询群信息失败！")
	}
	if group == nil {
		return errors.New("群不存在！")
	}
	realDeleteMemberModels, err := g.userDB.QueryByUIDs(req.Members)
	if err != nil {
		g.Error("查询成员用户信息失败！", zap.Error(err))
		return errors.New("查询成员用户信息失败！")
	}
	memberCount, err := g.db.QueryMemberCount(groupNo)
	if err != nil {
		g.Error("查询群成员数量失败！", zap.Error(err))
		return errors.New("查询群成员数量失败！")
	}

	userBaseVos := make([]*config.UserBaseVo, 0, len(realDeleteMemberModels))
	for _, realMember := range realDeleteMemberModels {
		userBaseVos = append(userBaseVos, &config.UserBaseVo{
			UID:  realMember.UID,
			Name: realMember.Name,
		})
	}
	nowMemberCount := int(memberCount) - len(userBaseVos) // 当前成员数量

	needGenGroupAvatar := false // 是否需要生成头像

	if nowMemberCount < 9 && nowMemberCount > 0 {
		needGenGroupAvatar = true
	}
	if !needGenGroupAvatar {
		needGenGroupAvatar, err = g.db.membersInFirstNine(groupNo, req.Members)
		if err != nil {
			g.Error("查询最早加入的成员信息失败！", zap.Error(err))
			return errors.New("查询最早加入的成员信息失败！")
		}
	}
	groupIsUploadAvatar, err := g.db.queryGroupAvatarIsUpload(groupNo)
	if err != nil {
		g.Error("查询群头像是否用户上传过失败！", zap.String("group_no", groupNo), zap.Error(err))
	}
	if groupIsUploadAvatar == 1 {
		needGenGroupAvatar = false
	}

	tx, err := g.db.session.Begin()
	util.CheckErr(err)
	defer func() {
		if err := recover(); err != nil {
			tx.RollbackUnlessCommitted()
			panic(err)
		}
	}()

	for _, realMember := range realDeleteMemberModels {

		version := g.ctx.GenSeq(common.GroupMemberSeqKey)
		err = g.db.DeleteMemberTx(groupNo, realMember.UID, version, tx)
		if err != nil {
			tx.RollbackUnlessCommitted()
			g.Error("删除群成员失败！", zap.Error(err))
			return errors.New("删除群成员失败！")
		}
	}

	// 发布群成员删除事件
	groupMemberRemoveReq := &config.MsgGroupMemberRemoveReq{
		GroupNo:      groupNo,
		Operator:     operator,
		OperatorName: operatorName,
		Members:      userBaseVos,
	}
	eventID, err := g.ctx.EventBegin(&wkevent.Data{
		Event: event.GroupMemberRemove,
		Type:  wkevent.Message,
		Data:  groupMemberRemoveReq,
	}, tx)
	if err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("开启事件失败！", zap.Error(err))
		return errors.New("开启事件失败！")
	}

	var groupAvatarEventID int64
	if needGenGroupAvatar {
		nineMemberUIDs := make([]string, 0, 9)
		nownineMembers, err := g.db.QueryMembersFirstNineExclude(groupNo, req.Members)
		if err != nil {
			tx.Rollback()
			g.Error("查询先存成员信息失败！", zap.String("group_no", groupNo), zap.Error(err))
			return errors.New("查询先存成员信息失败！")
		}
		if len(nownineMembers) > 0 {
			for _, nowninceMember := range nownineMembers {
				nineMemberUIDs = append(nineMemberUIDs, nowninceMember.UID)
			}
		}
		if len(nineMemberUIDs) > 0 {
			groupAvatarEventID, err = g.ctx.EventBegin(&wkevent.Data{
				Event: event.GroupAvatarUpdate,
				Type:  wkevent.CMD,
				Data: &config.CMDGroupAvatarUpdateReq{
					GroupNo: groupNo,
					Members: nineMemberUIDs,
				},
			}, tx)
			if err != nil {
				tx.Rollback()
				g.Error("开启群成员头像更新事件失败！", zap.Error(err))
				return errors.New("开启群成员头像更新事件失败！")
			}
		}
	}
	if err := tx.Commit(); err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("提交事务失败！", zap.Error(err))
		return errors.New("提交事务失败！")
	}
	// 提交事件
	g.ctx.EventCommit(eventID)
	if groupAvatarEventID != 0 {
		g.ctx.EventCommit(groupAvatarEventID)
	}

	// 调用IM的移除订阅者
	err = g.ctx.IMRemoveSubscriber(&config.SubscriberRemoveReq{
		ChannelID:   groupNo,
		ChannelType: common.ChannelTypeGroup.Uint8(),
		Subscribers: req.Members,
	})
	if err != nil {
		tx.RollbackUnlessCommitted()
		g.Error("调用IM的移除订阅者接口失败！", zap.Error(err))
		return errors.New("调用IM的移除订阅者接口失败！")
	}

	return nil
}
