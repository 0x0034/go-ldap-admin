package logic

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/eryajf/go-ldap-admin/config"
	"github.com/eryajf/go-ldap-admin/model"
	"github.com/eryajf/go-ldap-admin/public/tools"
	"github.com/eryajf/go-ldap-admin/service/ildap"
	"github.com/eryajf/go-ldap-admin/service/isql"
	"github.com/tidwall/gjson"
)

var (
	ReqAssertErr = tools.NewRspError(tools.SystemErr, fmt.Errorf("请求异常"))

	Api           = &ApiLogic{}
	User          = &UserLogic{}
	Group         = &GroupLogic{}
	Role          = &RoleLogic{}
	Menu          = &MenuLogic{}
	OperationLog  = &OperationLogLogic{}
	DingTalk      = &DingTalkLogic{}
	WeCom         = &WeComLogic{}
	FeiShu        = &FeiShuLogic{}
	Base          = &BaseLogic{}
	FieldRelation = &FieldRelationLogic{}
)

// CommonAddGroup 标准创建分组
func CommonAddGroup(group *model.Group) error {
	// 先在ldap中创建组
	err := ildap.Group.Add(group)
	if err != nil {
		return err
	}

	// 然后在数据库中创建组
	err = isql.Group.Add(group)
	if err != nil {
		return err
	}

	// 默认创建分组之后，需要将admin添加到分组中
	adminInfo := new(model.User)
	err = isql.User.Find(tools.H{"id": 1}, adminInfo)
	if err != nil {
		return err
	}

	err = isql.Group.AddUserToGroup(group, []model.User{*adminInfo})
	if err != nil {
		return err
	}

	return nil
}

// CommonUpdateGroup 标准更新分组
func CommonUpdateGroup(oldGroup, newGroup *model.Group) error {
	//若配置了不允许修改分组名称，则不更新分组名称
	if !config.Conf.Ldap.GroupNameModify {
		newGroup.GroupName = oldGroup.GroupName
	}

	err := ildap.Group.Update(oldGroup, newGroup)
	if err != nil {
		return err
	}
	err = isql.Group.Update(newGroup)
	if err != nil {
		return err
	}
	return nil
}

// CommonAddUser 标准创建用户
func CommonAddUser(user *model.User, groupId []uint) error {
	if user.Departments == "" {
		user.Departments = "默认:研发中心"
	}
	if user.GivenName == "" {
		user.GivenName = user.Nickname
	}
	if user.PostalAddress == "" {
		user.PostalAddress = "默认:地球"
	}
	if user.Position == "" {
		user.Position = "默认:技术"
	}
	if user.Introduction == "" {
		user.Introduction = user.Nickname
	}
	if user.JobNumber == "" {
		user.JobNumber = user.Mobile
	}
	// 先将用户添加到MySQL
	err := isql.User.Add(user)
	if err != nil {
		return tools.NewMySqlError(fmt.Errorf("向MySQL创建用户失败：" + err.Error()))
	}
	// 再将用户添加到ldap
	err = ildap.User.Add(user)
	if err != nil {
		return tools.NewLdapError(fmt.Errorf("AddUser向LDAP创建用户失败：" + err.Error()))
	}
	// 获取用户将要添加的分组
	groups, err := isql.Group.GetGroupByIds(groupId)
	if err != nil {
		return tools.NewMySqlError(fmt.Errorf("根据部门ID获取部门信息失败" + err.Error()))
	}
	for _, group := range groups {
		if group.GroupDN[:3] == "ou=" {
			continue
		}
		// 先将用户和部门信息维护到MySQL
		err := isql.Group.AddUserToGroup(group, []model.User{*user})
		if err != nil {
			return tools.NewMySqlError(fmt.Errorf("向MySQL添加用户到分组关系失败：" + err.Error()))
		}
		//根据选择的部门，添加到部门内
		err = ildap.Group.AddUserToGroup(group.GroupDN, user.UserDN)
		if err != nil {
			return tools.NewMySqlError(fmt.Errorf("向Ldap添加用户到分组关系失败：" + err.Error()))
		}
	}
	return nil
}

// CommonUpdateUser 标准更新用户
func CommonUpdateUser(oldUser, newUser *model.User, groupId []uint) error {
	// 更新用户
	if !config.Conf.Ldap.UserNameModify {
		newUser.Username = oldUser.Username
	}

	err := ildap.User.Update(oldUser.Username, newUser)
	if err != nil {
		return tools.NewLdapError(fmt.Errorf("在LDAP更新用户失败：" + err.Error()))
	}

	err = isql.User.Update(newUser)
	if err != nil {
		return tools.NewMySqlError(fmt.Errorf("在MySQL更新用户失败：" + err.Error()))
	}

	//判断部门信息是否有变化有变化则更新相应的数据库
	oldDeptIds := tools.StringToSlice(oldUser.DepartmentId, ",")
	addDeptIds, removeDeptIds := tools.ArrUintCmp(oldDeptIds, groupId)

	// 先处理添加的部门
	addgroups, err := isql.Group.GetGroupByIds(addDeptIds)
	if err != nil {
		return tools.NewMySqlError(fmt.Errorf("根据部门ID获取部门信息失败" + err.Error()))
	}
	for _, group := range addgroups {
		if group.GroupDN[:3] == "ou=" {
			continue
		}
		// 先将用户和部门信息维护到MySQL
		err := isql.Group.AddUserToGroup(group, []model.User{*newUser})
		if err != nil {
			return tools.NewMySqlError(fmt.Errorf("向MySQL添加用户到分组关系失败：" + err.Error()))
		}
		//根据选择的部门，添加到部门内
		err = ildap.Group.AddUserToGroup(group.GroupDN, newUser.UserDN)
		if err != nil {
			return tools.NewLdapError(fmt.Errorf("向Ldap添加用户到分组关系失败：" + err.Error()))
		}
	}

	// 再处理删除的部门
	removegroups, err := isql.Group.GetGroupByIds(removeDeptIds)
	if err != nil {
		return tools.NewMySqlError(fmt.Errorf("根据部门ID获取部门信息失败" + err.Error()))
	}
	for _, group := range removegroups {
		if group.GroupDN[:3] == "ou=" {
			continue
		}
		err := isql.Group.RemoveUserFromGroup(group, []model.User{*newUser})
		if err != nil {
			return tools.NewMySqlError(fmt.Errorf("在MySQL将用户从分组移除失败：" + err.Error()))
		}
		err = ildap.Group.RemoveUserFromGroup(group.GroupDN, newUser.UserDN)
		if err != nil {
			return tools.NewMySqlError(fmt.Errorf("在ldap将用户从分组移除失败：" + err.Error()))
		}
	}
	return nil
}

// BuildGroupData 根据数据与动态字段组装成分组数据
func BuildGroupData(flag string, remoteData map[string]interface{}) (*model.Group, error) {
	output, err := sonic.Marshal(&remoteData)
	if err != nil {
		return nil, err
	}

	oldData := new(model.FieldRelation)
	err = isql.FieldRelation.Find(tools.H{"flag": flag + "_group"}, oldData)
	if err != nil {
		return nil, tools.NewMySqlError(err)
	}
	frs, err := tools.JsonToMap(string(oldData.Attributes))
	if err != nil {
		return nil, tools.NewOperationError(err)
	}

	g := &model.Group{}
	for system, remote := range frs {
		switch system {
		case "groupName":
			g.SetGroupName(gjson.Get(string(output), remote).String())
		case "remark":
			g.SetRemark(gjson.Get(string(output), remote).String())
		case "sourceDeptId":
			g.SetSourceDeptId(fmt.Sprintf("%s_%s", flag, gjson.Get(string(output), remote).String()))
		case "sourceDeptParentId":
			g.SetSourceDeptParentId(fmt.Sprintf("%s_%s", flag, gjson.Get(string(output), remote).String()))
		}
	}
	return g, nil
}

// BuildUserData 根据数据与动态字段组装成用户数据
func BuildUserData(flag string, remoteData map[string]interface{}) (*model.User, error) {
	output, err := sonic.Marshal(&remoteData)
	if err != nil {
		return nil, err
	}

	fieldRelationSource := new(model.FieldRelation)
	err = isql.FieldRelation.Find(tools.H{"flag": flag + "_user"}, fieldRelationSource)
	if err != nil {
		return nil, tools.NewMySqlError(err)
	}
	fieldRelation, err := tools.JsonToMap(string(fieldRelationSource.Attributes))
	if err != nil {
		return nil, tools.NewOperationError(err)
	}

	u := &model.User{}
	for system, remote := range fieldRelation {
		switch system {
		case "username":
			u.SetUserName(gjson.Get(string(output), remote).String())
		case "nickname":
			u.SetNickName(gjson.Get(string(output), remote).String())
		case "givenName":
			u.SetGivenName(gjson.Get(string(output), remote).String())
		case "mail":
			u.SetMail(gjson.Get(string(output), remote).String())
		case "jobNumber":
			u.SetJobNumber(gjson.Get(string(output), remote).String())
		case "mobile":
			u.SetMobile(gjson.Get(string(output), remote).String())
		case "avatar":
			u.SetAvatar(gjson.Get(string(output), remote).String())
		case "postalAddress":
			u.SetPostalAddress(gjson.Get(string(output), remote).String())
		case "position":
			u.SetPosition(gjson.Get(string(output), remote).String())
		case "introduction":
			u.SetIntroduction(gjson.Get(string(output), remote).String())
		case "sourceUserId":
			u.SetSourceUserId(fmt.Sprintf("%s_%s", flag, gjson.Get(string(output), remote).String()))
		case "sourceUnionId":
			u.SetSourceUnionId(fmt.Sprintf("%s_%s", flag, gjson.Get(string(output), remote).String()))
		}
	}
	return u, nil
}

// ConvertDeptData 将部门信息转成本地结构体
func ConvertDeptData(flag string, remoteData []map[string]interface{}) (groups []*model.Group, err error) {
	for _, dept := range remoteData {
		group, err := BuildGroupData(flag, dept)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return
}

// ConvertUserData 将用户信息转成本地结构体
func ConvertUserData(flag string, remoteData []map[string]interface{}) (users []*model.User, err error) {
	for _, staff := range remoteData {
		groupIds, err := isql.Group.DeptIdsToGroupIds(staff["department_ids"].([]string))
		if err != nil {
			return nil, tools.NewMySqlError(fmt.Errorf("将部门ids转换为内部部门id失败：%s", err.Error()))
		}
		user, err := BuildUserData(flag, staff)
		if err != nil {
			return nil, err
		}
		user.DepartmentId = tools.SliceToString(groupIds, ",")
		users = append(users, user)
	}
	return
}
