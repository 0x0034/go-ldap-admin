package feishu

import (
	"context"
	"fmt"
	"strings"

	"github.com/chyroc/lark"
	"github.com/eryajf/go-ldap-admin/config"
	"github.com/mozillazg/go-pinyin"
)

// 官方文档： https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/department/children
// GetAllDepts 获取所有部门
func GetAllDepts() (ret []map[string]interface{}, err error) {
	var (
		fetchChild bool  = true
		pageSize   int64 = 50
	)

	req := lark.GetDepartmentListReq{
		FetchChild:   &fetchChild,
		PageSize:     &pageSize,
		DepartmentID: "0"}

	for {
		res, _, err := InitFeiShuClient().Contact.GetDepartmentList(context.TODO(), &req)
		if err != nil {
			return nil, err
		}
		for _, dept := range res.Items {
			ele := make(map[string]interface{})
			ele["name"] = dept.Name
			ele["name_pinyin"] = strings.Join(pinyin.LazyConvert(dept.Name, nil), "")
			ele["parent_department_id"] = dept.ParentDepartmentID
			ele["department_id"] = dept.DepartmentID
			ele["open_department_id"] = dept.OpenDepartmentID
			ele["leader_user_id"] = dept.LeaderUserID
			ele["unit_ids"] = dept.UnitIDs
			ret = append(ret, ele)
		}
		if !res.HasMore {
			break
		}
		req.PageToken = &res.PageToken
	}
	return
}

// 官方文档： https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/contact-v3/user/find_by_department
// GetAllUsers 获取所有员工信息
func GetAllUsers() (ret []map[string]interface{}, err error) {
	var (
		pageSize int64 = 50
	)
	depts, err := GetAllDepts()
	if err != nil {
		return nil, err
	}
	for _, dept := range depts {
		req := lark.GetUserListReq{
			PageSize:     &pageSize,
			PageToken:    new(string),
			DepartmentID: dept["department_id"].(string),
		}
		for {
			res, _, err := InitFeiShuClient().Contact.GetUserList(context.Background(), &req)
			if err != nil {
				return nil, err
			}
			for _, user := range res.Items {
				ele := make(map[string]interface{})
				ele["name"] = user.Name
				ele["name_pinyin"] = strings.Join(pinyin.LazyConvert(user.Name, nil), "")
				ele["union_id"] = user.UnionID
				ele["user_id"] = user.UserID
				ele["open_id"] = user.OpenID
				ele["en_name"] = user.EnName
				ele["nickname"] = user.Nickname
				ele["email"] = user.Email
				ele["mobile"] = user.Mobile[3:]
				ele["gender"] = user.Gender
				ele["avatar"] = user.Avatar.AvatarOrigin
				ele["city"] = user.City
				ele["country"] = user.Country
				ele["work_station"] = user.WorkStation
				ele["join_time"] = user.JoinTime
				ele["employee_no"] = user.EmployeeNo
				ele["enterprise_email"] = user.EnterpriseEmail
				ele["job_title"] = user.JobTitle
				// 部门ids
				var sourceDeptIds []string
				for _, deptId := range user.DepartmentIDs {
					sourceDeptIds = append(sourceDeptIds, fmt.Sprintf("%s_%s", config.Conf.FeiShu.Flag, deptId))
				}
				ele["department_ids"] = sourceDeptIds
				ret = append(ret, ele)
			}
			if !res.HasMore {
				break
			}
			req.PageToken = &res.PageToken
		}
	}
	return
}

// 官方文档： https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/ehr/ehr-v1/employee/list
// GetLeaveUserIds 获取离职人员ID列表
func GetLeaveUserIds() ([]string, error) {
	var ids []string
	users, _, err := InitFeiShuClient().EHR.GetEHREmployeeList(context.TODO(), &lark.GetEHREmployeeListReq{
		Status:     []int64{5},
		UserIDType: lark.IDTypePtr(lark.IDTypeUnionID), // 只查询unionID
	})
	if err != nil {
		return nil, err
	}
	for _, user := range users.Items {
		ids = append(ids, user.UserID)
	}
	return ids, nil
}
