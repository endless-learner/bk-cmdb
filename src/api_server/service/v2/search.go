/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v2

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"configcenter/src/api_server/logics/v2/common/converter"
	"configcenter/src/api_server/logics/v2/common/defs"
	"configcenter/src/api_server/logics/v2/common/utils"
	"configcenter/src/common"
	"configcenter/src/common/blog"
	ccErr "configcenter/src/common/errors"
	"configcenter/src/common/mapstr"
	"configcenter/src/common/metadata"
	"configcenter/src/common/util"

	"github.com/emicklei/go-restful"
)

func (s *Service) getModuleInfoByApp(appID int64, pheader http.Header) (map[int64]mapstr.MapStr, ccErr.CCError) {
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))
	rid := util.GetHTTPCCRequestID(pheader)

	moduleMap := make(map[int64]mapstr.MapStr)

	//set empty to get all fields
	param := mapstr.MapStr{
		"fields":    []string{},
		"condition": make(map[string]interface{}),
		"page": map[string]interface{}{
			"start": 0,
			"limit": 0,
		},
	}

	result, err := s.CoreAPI.TopoServer().OpenAPI().SearchModuleByApp(context.Background(), strconv.FormatInt(appID, 10), pheader, param)
	if err != nil {
		blog.Errorf("convert module res to v2  error:%v, query:%+v,rid:%s", err, param, rid)
		return nil, err
	}

	if false == result.Result {
		return nil, defErr.New(result.Code, result.ErrMsg)
	}

	for _, module := range result.Data.Info {
		moduleId, err := module.Int64(common.BKModuleIDField)
		if nil != err {
			continue
		}
		moduleMap[moduleId] = module
	}
	return moduleMap, nil
}

func (s *Service) getIPAndProxyByCompany(req *restful.Request, resp *restful.Response) {

	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	err := req.Request.ParseForm()

	if err != nil {
		blog.Errorf("getIPAndProxyByCompany Error %v", err)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form
	appID := formData.Get("appId")
	platID := formData.Get("platId")
	ips := formData.Get("ipList")
	if "" == appID {
		blog.Errorf("getIPAndProxyByCompany error appID empty")
		converter.RespFailV2(common.CCErrCommParamsNeedSet, defErr.Errorf(common.CCErrCommParamsNeedSet, "appId").Error(), resp)
		return
	}
	if "" == platID {
		blog.Errorf("getIPAndProxyByCompany error platID empty")
		converter.RespFailV2(common.CCErrCommParamsNeedSet, defErr.Errorf(common.CCErrCommParamsNeedSet, "platId").Error(), resp)
		return
	}
	if "" == ips {
		blog.Errorf("getIPAndProxyByCompany error ipList empty")
		converter.RespFailV2(common.CCErrCommParamsNeedSet, defErr.Errorf(common.CCErrCommParamsNeedSet, "ipList").Error(), resp)
		return
	}
	ipArr := strings.Split(ips, ",")
	input := make(common.KvMap)
	input["ips"] = ipArr
	input[common.BKAppIDField] = appID
	input[common.BKCloudIDField] = platID
	result, err := s.CoreAPI.HostServer().GetIPAndProxyByCompany(context.Background(), pheader, input)
	if err != nil {
		blog.Errorf("getIPAndProxyByCompany  error:%s ", err.Error())
		converter.RespFailV2(common.CCErrCommHTTPDoRequestFailed, defErr.Error(common.CCErrCommHTTPDoRequestFailed).Error(), resp)
		return
	}

	if !result.Result {
		converter.RespFailV2(common.CCErrCommJSONMarshalFailed, defErr.Error(common.CCErrCommJSONMarshalFailed).Error(), resp)
		return
	}

	converter.RespSuccessV2(result.Data, resp)
}

func (s *Service) getHostListByIP(req *restful.Request, resp *restful.Response) {
	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))
	rid := util.GetHTTPCCRequestID(pheader)

	err := req.Request.ParseForm()
	if err != nil {
		blog.Errorf("getHostListByIP error:%v,rid:%s", err, rid)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form

	if len(formData["IP"]) == 0 || formData["IP"][0] == "" {
		blog.Errorf("getHostListByIP error: param IP is empty!")
		converter.RespFailV2(common.CCErrCommParamsNeedSet, defErr.Errorf(common.CCErrCommParamsNeedSet, "IP").Error(), resp)
		return
	}

	ipArr := strings.Split(formData["IP"][0], ",")

	//build v3 params
	param := &metadata.HostSearchByIPParams{
		IpList: ipArr,
	}
	// param := map[string]interface{}{
	// 	common.BKIPListField: ipArr,
	// }
	if len(formData["ApplicationID"]) > 0 {
		appIDStrArr := strings.Split(formData["ApplicationID"][0], ",")
		appIDArr, sliceErr := utils.SliceStrToInt(appIDStrArr)
		if nil != sliceErr {
			blog.Errorf("getHostListByIP error: %v,input:%+v,rid:%s", sliceErr, formData, rid)
			converter.RespFailV2(common.CCErrCommParamsNeedSet, defErr.Errorf(common.CCErrCommParamsNeedSet, "ApplicationID").Error(), resp)
			return
		}
		param.AppID = appIDArr
	}

	if len(formData["platID"]) > 0 {
		platIDStr := formData["platID"][0]
		platID, err := util.GetInt64ByInterface(platIDStr)
		if nil != err {
			blog.Errorf("getHostListByIP error: %v, input:%+v,rid:%s", err, formData, rid)
			converter.RespFailV2Error(defErr.Errorf(common.CCErrCommParamsNeedInt, "platID"), resp)
			return
		}
		param.CloudID = &platID

	}

	result, err := s.CoreAPI.HostServer().HostSearchByIP(context.Background(), pheader, param)
	if err != nil {
		blog.Errorf("getHostListByIP  error:%v,input:%+v,rid:%s", err, formData, rid)
		converter.RespFailV2Error(defErr.Error(common.CCErrCommHTTPDoRequestFailed), resp)
		return
	}
	if !result.Result {
		blog.Errorf("getHostListByIP http response error. err code:%d,err msg:%s ,input:%+v,rid:%s", result.Code, result.ErrMsg, formData, rid)
		converter.RespFailV2(result.Code, result.ErrMsg, resp)
		return
	}
	resDataV2, err := converter.ResToV2ForHostList(result.Result, result.ErrMsg, result.Data)
	if err != nil {
		blog.Errorf("convert host res to v2 error:%v,input:%+v,rid:%s", err, formData, rid)
		converter.RespFailV2(common.CCErrCommReplyDataFormatError, defErr.Error(common.CCErrCommReplyDataFormatError).Error(), resp)
		return
	}
	converter.RespSuccessV2(resDataV2, resp)

}

func (s *Service) getSetHostList(req *restful.Request, resp *restful.Response) {

	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	err := req.Request.ParseForm()
	if err != nil {
		blog.Errorf("getSetHostList error:%v", err)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form

	res, msg := utils.ValidateFormData(formData, []string{"ApplicationID", "SetID"})
	if !res {
		blog.Errorf("getSetHostList error: %s", msg)
		converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
		return
	}

	appID, err := strconv.Atoi(formData["ApplicationID"][0])
	if nil != err {
		blog.Errorf("getSetHostList error: %v", err)
		converter.RespFailV2(common.CCErrCommParamsNeedInt, defErr.Errorf(common.CCErrCommParamsNeedInt, "ApplicationID").Error(), resp)
		return
	}

	setIDStrArr := strings.Split(formData["SetID"][0], ",")
	setIDArr, err := utils.SliceStrToInt(setIDStrArr)
	if nil != err {
		blog.Errorf("getSetHostList error: %v", err)
		converter.RespFailV2(common.CCErrAPIServerV2MultiSetIDErr, defErr.Error(common.CCErrAPIServerV2MultiSetIDErr).Error(), resp)
		return
	}

	param := map[string]interface{}{
		common.BKAppIDField: appID,
		common.BKSetIDField: setIDArr,
	}

	result, err := s.CoreAPI.HostServer().HostSearchBySetID(context.Background(), pheader, param)
	if err != nil {
		blog.Errorf("getSetHostList error:%v", err)
		converter.RespFailV2(common.CCErrCommHTTPDoRequestFailed, defErr.Error(common.CCErrCommHTTPDoRequestFailed).Error(), resp)
		return
	}

	resDataV2, err := converter.ResToV2ForHostList(result.Result, result.ErrMsg, result.Data)
	if err != nil {
		blog.Errorf("convert host res to v2 error:%v", err)
		converter.RespFailV2(common.CCErrCommReplyDataFormatError, defErr.Error(common.CCErrCommReplyDataFormatError).Error(), resp)
		return
	}

	converter.RespSuccessV2(resDataV2, resp)
}

func (s *Service) getModuleHostList(req *restful.Request, resp *restful.Response) {

	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	err := req.Request.ParseForm()
	if err != nil {
		blog.Errorf("getModuleHostList error:%v", err)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form

	res, msg := utils.ValidateFormData(formData, []string{"ApplicationID", "ModuleID"})
	if !res {
		blog.Errorf("getModuleHostList error: %s", msg)
		converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
		return
	}

	appID, err := strconv.Atoi(formData["ApplicationID"][0])
	if nil != err {
		blog.Errorf("getModuleHostList error: %v", err)
		converter.RespFailV2(common.CCErrCommParamsNeedInt, defErr.Errorf(common.CCErrCommParamsNeedInt, "ApplicationID").Error(), resp)
		return
	}

	moduleIDStrArr := strings.Split(formData["ModuleID"][0], ",")
	moduleIDArr, err := utils.SliceStrToInt(moduleIDStrArr)
	if nil != err {
		blog.Errorf("getModuleHostList error: %v", err)
		converter.RespFailV2(common.CCErrAPIServerV2MultiModuleIDErr, defErr.Error(common.CCErrAPIServerV2MultiModuleIDErr).Error(), resp)
		return
	}

	param := map[string]interface{}{
		common.BKAppIDField:    appID,
		common.BKModuleIDField: moduleIDArr,
	}

	result, err := s.CoreAPI.HostServer().HostSearchByModuleID(context.Background(), pheader, param)

	if err != nil {
		blog.Errorf("getModuleHostList list  error:%v", err)
		converter.RespFailV2(common.CCErrCommHTTPDoRequestFailed, defErr.Error(common.CCErrCommHTTPDoRequestFailed).Error(), resp)
		return
	}

	resDataV2, err := converter.ResToV2ForHostList(result.Result, result.ErrMsg, result.Data)
	if err != nil {
		blog.Errorf("convert host res to v2 error:%v", err)
		converter.RespFailV2(common.CCErrCommReplyDataFormatError, defErr.Error(common.CCErrCommReplyDataFormatError).Error(), resp)
		return
	}

	converter.RespSuccessV2(resDataV2, resp)
}

func (s *Service) getAppHostList(req *restful.Request, resp *restful.Response) {

	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	err := req.Request.ParseForm()
	if err != nil {
		blog.Errorf("getAppHostList error:%v", err)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form

	blog.Infof("getAppHostList data: %v", formData)

	res, msg := utils.ValidateFormData(formData, []string{"ApplicationID"})
	if !res {
		blog.Errorf("getAppHostList error: %s", msg)
		converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
		return
	}

	appID, err := strconv.Atoi(formData["ApplicationID"][0])
	if nil != err {
		blog.Errorf("getAppHostList error: %v", err)
		converter.RespFailV2(common.CCErrCommParamsNeedInt, defErr.Errorf(common.CCErrCommParamsNeedInt, "ApplicationID").Error(), resp)
		return
	}

	param := map[string]interface{}{
		common.BKAppIDField: appID,
	}
	result, err := s.CoreAPI.HostServer().HostSearchByAppID(context.Background(), pheader, param)

	if err != nil {
		blog.Errorf("getAppHostList  error:%v", err)
		converter.RespFailV2(common.CCErrCommHTTPDoRequestFailed, defErr.Error(common.CCErrCommHTTPDoRequestFailed).Error(), resp)
		return
	}

	resDataV2, err := converter.ResToV2ForHostList(result.Result, result.ErrMsg, result.Data)
	if err != nil {
		blog.Errorf("convert host res to v2 error:%v", err)
		converter.RespFailV2(common.CCErrCommReplyDataFormatError, defErr.Error(common.CCErrCommReplyDataFormatError).Error(), resp)
		return
	}

	blog.Infof("getAppHostList success, data length: %d", len(resDataV2.([]interface{})))

	converter.RespSuccessV2(resDataV2, resp)
}

func (s *Service) getHostsByProperty(req *restful.Request, resp *restful.Response) {

	pheader := req.Request.Header
	defErr := s.CCErr.CreateDefaultCCErrorIf(util.GetLanguage(pheader))

	err := req.Request.ParseForm()
	if err != nil {
		blog.Errorf("getHostsByProperty error:%v", err)
		converter.RespFailV2(common.CCErrCommPostInputParseError, defErr.Error(common.CCErrCommPostInputParseError).Error(), resp)
		return
	}

	formData := req.Request.Form

	res, msg := utils.ValidateFormData(formData, []string{"ApplicationID"})
	if !res {
		blog.Errorf("getHostsByProperty error: %s", msg)
		converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
		return
	}

	appID, err := strconv.Atoi(formData["ApplicationID"][0])
	if nil != err {
		blog.Errorf("getHostsByProperty error: %v", err)
		converter.RespFailV2(common.CCErrCommParamsNeedInt, defErr.Errorf(common.CCErrCommParamsNeedInt, "ApplicationID").Error(), resp)
		return
	}

	//build v3 params
	param := map[string]interface{}{
		common.BKAppIDField: appID,
	}

	if len(formData["SetID"]) > 0 && "" != formData["SetID"][0] {
		setIDArr, sliceErr := utils.SliceStrToInt(strings.Split(formData["SetID"][0], ","))
		if nil != sliceErr {
			blog.Errorf("getHostsByProperty error: %v", sliceErr)
			converter.RespFailV2(common.CCErrAPIServerV2MultiSetIDErr, defErr.Errorf(common.CCErrAPIServerV2MultiSetIDErr).Error(), resp)
			return
		}
		param[common.BKSetIDField] = setIDArr
	}

	if len(formData["SetEnviType"]) > 0 && "" != formData["SetEnviType"][0] {

		setEnvArrTemp := strings.Split(formData["SetEnviType"][0], ",")
		setEnvArr := make([]string, 0)
		for _, setEnv := range setEnvArrTemp {
			setEnvV3, ok := defs.SetEnvMap[setEnv]
			if !ok {
				msg := fmt.Sprintf("SetEnviType not in 1,2,3, it is %s", setEnv)
				blog.Error(msg)
				converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
				return
			}
			setEnvArr = append(setEnvArr, setEnvV3)
		}
		param[common.BKSetEnvField] = setEnvArr
	}

	if len(formData["SetServiceStatus"]) > 0 && "" != formData["SetServiceStatus"][0] {

		setStatusArrTemp := strings.Split(formData["SetServiceStatus"][0], ",")
		setStatusArr := make([]string, 0)
		for _, setStatus := range setStatusArrTemp {
			setStatusV3, ok := defs.SetStatusMap[setStatus]
			if !ok {
				msg := fmt.Sprintf("SetServiceStatus not in 0,1, it is %s", setStatus)
				blog.Error(msg)
				converter.RespFailV2(common.CCErrAPIServerV2DirectErr, defErr.Errorf(common.CCErrAPIServerV2DirectErr, msg).Error(), resp)
				return
			}
			setStatusArr = append(setStatusArr, setStatusV3)
		}

		param[common.BKSetStatusField] = setStatusArr
	}

	result, err := s.CoreAPI.HostServer().HostSearchByProperty(context.Background(), pheader, param)
	if err != nil {
		blog.Errorf("getHostsByProperty  error:%v", err)
		converter.RespFailV2(common.CCErrCommHTTPDoRequestFailed, defErr.Error(common.CCErrCommHTTPDoRequestFailed).Error(), resp)
		return
	}

	resDataV2, err := converter.ResToV2ForHostList(result.Result, result.ErrMsg, result.Data)
	if err != nil {
		blog.Errorf("convert host res to v2 error:%v", err)
		converter.RespFailV2(common.CCErrCommReplyDataFormatError, defErr.Error(common.CCErrCommReplyDataFormatError).Error(), resp)
		return
	}

	converter.RespSuccessV2(resDataV2, resp)
}
