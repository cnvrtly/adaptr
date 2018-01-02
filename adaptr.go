package adaptr

import (
	"net/http"
	"context"
	"sync"
	xCtx "golang.org/x/net/context"
	"encoding/json"
	"io/ioutil"
	"bytes"
	"github.com/julienschmidt/httprouter"
	"fmt"
)

type Adapter func(handle http.Handler) http.Handler

func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for i := len(adapters) - 1; i >= 0; i-- {
		h = adapters[i](h)
	}
	return h
}

type NewPlatformCtx func(*http.Request) context.Context
type NewPlatformXCtx func(*http.Request) xCtx.Context

func PlatformXCtxAdapter(NewContextFn NewPlatformXCtx) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			fmt.Println("JJJJJJJ2")
			h.ServeHTTP(w, r.WithContext(NewContextFn(r)))
		})
	}
}

func PlatformCtxAdapter(NewContextFn NewPlatformCtx) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(NewContextFn(r)))
		})
	}
}

func SetCtxValue(r *http.Request, key, value interface{}) (*http.Request) {
	return r.WithContext(context.WithValue(r.Context(), key, value))
}

func GetCtxValue(r *http.Request, key interface{}) interface{} {
	return r.Context().Value(key)
}

func GetCtxValueStr(r *http.Request, key interface{}) string {
	ctxVal := GetCtxValue(r, key)
	if ctxVal == nil {
		return ""
	}
	return ctxVal.(string)
}

func CallOnce(f func(w http.ResponseWriter, r *http.Request)) Adapter {
	once := sync.Once{}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			once.Do(func() {
				f(w, r)
			})
			h.ServeHTTP(w, r)
		})
	}
}

func JsonContentType() Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			h.ServeHTTP(w, r)
		})
	}
}

func AuthBouncer(checkAuthorizedCtxKey interface{}) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Context().Value(checkAuthorizedCtxKey) == true {
				if h != nil {
					h.ServeHTTP(w, r)
				}
			} else {
				http.Error(w, "Not authorized", http.StatusForbidden)
			}
		})
	}
}

func Cors(domain string, allowHeaders ... string) Adapter {
	allowAll := domain == ""
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowAll {
				domain = r.Header.Get("origin")
			}
			w.Header().Set("Access-Control-Allow-Origin", domain)
			for _, hdr := range allowHeaders {
				w.Header().Add("Access-Control-Allow-Headers", hdr)
			}
			w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
			h.ServeHTTP(w, r)
		})
	}
}

func ParamId2Ctx(ctxKey interface{}) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params:=GetCtxValue(r, CtxHttpRouterParamsKey)
			if params!= nil {
				idVal:=params.(httprouter.Params).ByName("id")
				if idVal!= "" {
					SetCtxValue(r, ctxKey, idVal)
				}
			}
		})
	}
}

func Json2Ctx(ctxKey interface{}, reset bool, requiredProps ... string) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
fmt.Println("JJJJJJJ")
			if currCtxVal := GetCtxValue(r, ctxKey); !reset && currCtxVal != nil {
				for _, param := range requiredProps {
					if _, ok := currCtxVal.(map[string]interface{})[param]; !ok {
						http.Error(w, fmt.Sprintf("Missing required JSON property name=%v", param), http.StatusBadRequest)
						return
					}
				}
				///return
			} else {

				fmt.Println("JJJJJJJ1")
				valueStructPointer := map[string]interface{}{}
				if (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
					bodyValCtxRequest, err := getBodyValue(r)
					if err != nil {
						if requiredProps!=nil && len(requiredProps)>0 {
							http.Error(w, fmt.Sprintf("Error getting request body err=%v", err), http.StatusBadRequest)
							return
						}
						h.ServeHTTP(w, r)
						return
					}
					if bodyValCtxRequest != nil {
						r=bodyValCtxRequest
					}
					bodyVal:= GetCtxValue(r, CtxRequestBodyByteArrKey).([]byte)
					if bodyVal == nil || len(bodyVal)==0 {
						if requiredProps!=nil && len(requiredProps)>0 {
							http.Error(w, "Please send a request body", http.StatusBadRequest)
							return
						}
						h.ServeHTTP(w, r)
						return
					}
					err = json.NewDecoder(bytes.NewBuffer(bodyVal)).Decode(&valueStructPointer)
					if err != nil {
						http.Error(w, "error parsing json err="+err.Error(), http.StatusBadRequest)
						return
					}
					for _, param := range requiredProps {
						if _, ok := valueStructPointer[param]; !ok {
							http.Error(w, fmt.Sprintf("Missing required JSON property=%v", param), http.StatusBadRequest)
							return
						}
					}
				}
				if r.Method == http.MethodGet && len(requiredProps)>0{
					for _, param := range requiredProps {

						paramVal := r.URL.Query().Get(param)
						if paramVal == "" {
							http.Error(w, fmt.Sprintf("Missing required url param=%v", param), http.StatusBadRequest)
							return
						}

						valueStructPointer[param] = paramVal
					}
				}
				/*///if r.Body == nil {
					http.Error(w, "Please send a request body", http.StatusBadRequest)
					return
				}*/

				//if valueStructPointer == nil {
				///valueStructPointer := map[string]interface{}{}
				//}
				if (len(valueStructPointer) > 0) {
					r = SetCtxValue(r, ctxKey, valueStructPointer)
				}
			}
			h.ServeHTTP(w, r)
		})
	}
}



func ReqrdParams(reqMethod string, requiredParams ... string) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, param := range requiredParams {
				switch reqMethod {
				case http.MethodGet:
					if r.URL.Query().Get(param) == "" {
						http.Error(w, fmt.Sprintf("Missing required url parameter=%v", param), http.StatusBadRequest)
						return
					}
				case http.MethodPost, http.MethodPut:

					if r.FormValue(param) == "" {
						hah, _ := ioutil.ReadAll(r.Body);
						//defer r.Body.Close()
						//r.Body.Read(str)
						http.Error(w, fmt.Sprintf("Missing required body parameter=%v val=%v", param, string(hah)), http.StatusBadRequest)
						return
					}
				}
			}
			h.ServeHTTP(w, r)
		})
	}
}

func ValidateCtxTkn(ctxTokenKey interface{}, tknValidationFunc func(tkn string) (bool, error)) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//check token

			var isValid bool = false
			var err error
			ctxTknVal := GetCtxValue(r, ctxTokenKey)
			if ctxTknVal != nil {
				var tknValue string = ctxTknVal.(string)
				if tknValue != "" {

					isValid, err = tknValidationFunc(tknValue)
					if err != nil {
						//log.Errorf(r.Context(), "ValidateCtxTkn Adapter nativeTS.Validate err=", err)
						http.Error(w, "Token not valid.", http.StatusUnauthorized)
						return
					}

					if isValid {
						h.ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w, "Authorization token not valid or not present", http.StatusUnauthorized)
			return
		})
	}
}

func Tkn2Ctx(ctxTokenKey interface{}, tknParameterName string, requestJsonStructCtxKey interface{}) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tknValue, err := GetTokenFromReq(r, tknParameterName, requestJsonStructCtxKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			r = SetCtxValue(r, ctxTokenKey, tknValue)

			h.ServeHTTP(w, r)
		})
	}
}

func AuthPermitAll(ctxRouteAuthorizedKey interface{}) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ctxRouteAuthorizedKey == nil {
				ctxRouteAuthorizedKey = CtxRouteAuthorizedKey
			}
			h.ServeHTTP(w, SetCtxValue(r, ctxRouteAuthorizedKey, true))
		})
	}
}

func WriteResponse(writeValue string) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//_,err:=w.Write(writeValue)
			_, err := fmt.Fprintln(w, writeValue)
			if err != nil {
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}

func getBodyValue(r *http.Request) (*http.Request, error) {
	bodyVal := GetCtxValue(r, CtxRequestBodyByteArrKey)
	if bodyVal != nil {
		return nil, nil
	}

	bodyValRead, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	return SetCtxValue(r, CtxRequestBodyByteArrKey, bodyValRead), nil
}
