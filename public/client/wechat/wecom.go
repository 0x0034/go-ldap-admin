package wechat

import (
	"fmt"
	"strings"

	"github.com/eryajf/go-ldap-admin/config"
	"github.com/mozillazg/go-pinyin"
	"github.com/wenerme/go-wecom/wecom"
)

// 官方文档： https://developer.work.weixin.qq.com/document/path/90208
// GetAllDepts 获取所有部门
func GetAllDepts() (ret []map[string]interface{}, err error) {
	depts, err := InitWeComClient().ListDepartment(
		&wecom.ListDepartmentRequest{},
	)
	if err != nil {
		return nil, err
	}
	for _, dept := range depts.Department {
		ele := make(map[string]interface{})
		ele["name"] = dept.Name
		ele["name_pinyin"] = strings.Join(pinyin.LazyConvert(dept.Name, nil), "")
		ele["id"] = dept.ID
		ele["name_en"] = dept.NameEn
		ele["parentid"] = dept.ParentID
		ret = append(ret, ele)
	}
	return ret, nil
}

// 官方文档： https://developer.work.weixin.qq.com/document/path/90201
// GetAllUsers 获取所有员工信息
func GetAllUsers() (ret []map[string]interface{}, err error) {
	depts, err := GetAllDepts()
	if err != nil {
		return nil, err
	}
	for _, dept := range depts {
		users, err := InitWeComClient().ListUser(
			&wecom.ListUserRequest{
				DepartmentID: fmt.Sprintf("%d", dept["id"].(int)),
				FetchChild:   "1",
			},
		)
		if err != nil {
			return nil, err
		}
		for _, user := range users.UserList {
			ele := make(map[string]interface{})
			ele["name"] = user.Name
			ele["name_pinyin"] = strings.Join(pinyin.LazyConvert(user.Name, nil), "")
			ele["userid"] = user.UserID
			ele["mobile"] = user.Mobile
			ele["position"] = user.Position
			ele["gender"] = user.Gender
			ele["email"] = user.Email
			ele["biz_email"] = user.BizMail
			ele["avatar"] = user.Avatar
			ele["telephone"] = user.Telephone
			ele["alias"] = user.Alias
			ele["external_position"] = user.ExternalPosition
			ele["address"] = user.Address
			ele["open_userid"] = user.OpenUserID
			ele["main_department"] = user.MainDepartment
			ele["english_name"] = user.EnglishName
			// 部门ids
			var sourceDeptIds []string
			for _, deptId := range user.Department {
				sourceDeptIds = append(sourceDeptIds, fmt.Sprintf("%s_%d", config.Conf.FeiShu.Flag, deptId))
			}
			ele["department_ids"] = sourceDeptIds
			ret = append(ret, ele)
		}
	}
	return ret, nil
}
