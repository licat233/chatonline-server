package visitor

import (
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/gocraft/dbr/v2"
)

type Model struct {
	Id          int64     `db:"id"`            // 表主键
	Uuid        string    `db:"uuid"`          // UUID
	UserUid     string    `db:"user_uid"`      // 用户uid
	KefuUid     string    `db:"kefu_uid"`      // 客服uid
	ProjectUid  string    `db:"project_uid"`   // 项目uid
	Comment     string    `db:"comment"`       // 备注
	Status      int64     `db:"status"`        // 状态:0离线，1在线，2拉黑
	Name        string    `db:"name"`          // 游客名
	Ip          string    `db:"ip"`            // IP
	Referer     string    `db:"referer"`       // 来源，从哪个网址发起的聊天
	Region      string    `db:"region"`        // 地区
	City        string    `db:"city"`          // 城市
	Platform    string    `db:"platform"`      // 平台:移动设备,电脑,平板,Xbox
	Device      string    `db:"device"`        // 设备:移动设备,电脑,平板,Xbox
	Os          string    `db:"os"`            // 系统: andriod,ios,macOS,windows
	Browser     string    `db:"browser"`       // 浏览器: Chrome,Safari,InternalExplorer,Firefox,Edge
	Language    string    `db:"language"`      // 语言
	LastLoginAt time.Time `db:"last_login_at"` // 最后一次登录时间
	CreateAt    time.Time `db:"create_at"`     // 创建时间
	UpdateAt    time.Time `db:"update_at"`     // 更新时间
}

// DB 游客db操作
type DB struct {
	session *dbr.Session
	ctx     *config.Context
}

// New NewDB
func NewDB(ctx *config.Context) *DB {
	return &DB{
		session: ctx.DB(),
		ctx:     ctx,
	}
}

// QueryByUUID 通过用户uuid查询用户信息
func (d *DB) QueryByUUID(uuid string) (*Model, error) {
	if strings.TrimSpace(uuid) == "" {
		return nil, nil
	}
	var model *Model
	_, err := d.session.Select("*").From("visitor").Where("uuid=? and uuid<>''", uuid).Load(&model)
	return model, err
}

func (d *DB) QueryByUIDs(uuids []string) ([]*Model, error) {
	if len(uuids) <= 0 {
		return nil, nil
	}
	var models []*Model
	_, err := d.session.Select("*").From("visitor").Where("uuid in ?", uuids).Load(&models)
	return models, err
}

// QueryByName 通过用户名查询用户信息
func (d *DB) QueryByName(name string) (*Model, error) {
	var model *Model
	_, err := d.session.Select("*").From("visitor").Where("name=? and name<>''", name).Load(&model)
	return model, err
}

// UpdateName 修改用户名
func (d *DB) UpdateName(name string, uuid string) error {
	_, err := d.session.Update("visitor").Set("name", name).Where("uuid=? and uuid<>''", uuid).Exec()
	return err
}

// UpdateComment 修改备注
func (d *DB) UpdateComment(comment string, uuid string) error {
	_, err := d.session.Update("visitor").Set("comment", comment).Where("uuid=? and uuid<>''", uuid).Exec()
	return err
}

// Insert 添加游客
func (d *DB) Insert(m *Model) error {
	_, err := d.session.InsertInto("visitor").Columns(util.AttrToUnderscore(m)...).Record(m).Exec()
	return err
}

// Update 更新游客
func (d *DB) Update(model *Model) error {
	_, err := d.session.Update("visitor").SetMap(map[string]interface{}{
		"kefu_uid":      model.KefuUid,
		"name":          model.Name,
		"comment":       model.Comment,
		"status":        model.Status,
		"ip":            model.Ip,
		"referer":       model.Referer,
		"region":        model.Region,
		"city":          model.City,
		"platform":      model.Platform,
		"device":        model.Device,
		"os":            model.Os,
		"browser":       model.Browser,
		"language":      model.Language,
		"last_login_at": model.LastLoginAt.Format("2006-01-02 15:04:05"),
	}).Where("id=?", model.Id).Exec()
	return err
}
