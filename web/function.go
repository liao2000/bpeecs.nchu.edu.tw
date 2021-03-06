package web

import(
    "fmt"
    "encoding/json"
    "net/http"
    "regexp"
    "strconv"
    "bpeecs.nchu.edu.tw/article"
    "bpeecs.nchu.edu.tw/login"
    "bpeecs.nchu.edu.tw/function"
    "bpeecs.nchu.edu.tw/files"
)

type attachmentJSONStruct struct{
    ClientName string `json:"client_name"`
    Path string `json:"path"`
    ServerName string `json:"server_name"`
}

func attachmentJSONtoClientName(attachmentJSON string) []string{
    attachment := []attachmentJSONStruct{}
    json.Unmarshal([]byte(attachmentJSON), &attachment)
    serverNameList := []string{}
    for _, v := range attachment{
        serverNameList = append(serverNameList, v.ServerName)
    }
    return serverNameList
}


// FunctionWebHandler is a handler for handling url whose prefix is /function
func FunctionWebHandler(w http.ResponseWriter, r *http.Request){
    r.ParseForm()
    path := r.URL.Path

    if path == "/function/login" {
        l := login.New()

        if err := l.Login(w, r); err != nil{
            fmt.Fprint(w, err.Error())
            return
        }

        fmt.Fprint(w, `{"err" : false}`)

        return
    }else if path == "/function/reg" {
        // only current accounts are allowed to register a new account
        loginInfo := login.CheckLogin(w, r)
        if loginInfo == nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "必需登入才能建立新帳戶(基於網路安全)"}`)
            return
        }

        l := login.New()

        id := function.GET("id", r)
        pwd := function.GET("pwd", r)
        rePwd := function.GET("re_pwd", r)
        name := function.GET("name", r)

        match, err := regexp.MatchString("^[a-zA-Z0-9_]{5,30}$", id)
        if err!=nil || !match {
            fmt.Fprint(w, `{"err" : true , "msg" : "帳號僅接受「英文字母、數字、-、_」且需介於 5 到 30 字元`)
            return
        }

        if len(name) > 30 || len(name) < 1{
            fmt.Fprint(w, `{"err" : true , "msg" : "暱稱需介於 1 到 30 字元"}`)
            return
        }

        if pwd != rePwd{
            fmt.Fprint(w, `{"err" : true , "msg" : "密碼與確認密碼不一致"}`)
            return
        }

        match, err = regexp.MatchString("^[a-zA-Z0-9_]{8,30}$", pwd)
        if err != nil || !match {
            fmt.Fprint(w, `{"err" : true , "msg" : "密碼僅接受「英文字母、數字、-、_」且需介於 8 到 30 字元"}`)
            return
        }

        match, err = regexp.MatchString("^.*?\\d+.*?$", pwd)
        if err != nil || !match {
            fmt.Fprint(w, `{"err" : true , "msg" : "密碼必需含有數字"}`)
            return
        }

        match, err = regexp.MatchString("^.*?[a-zA-Z]+.*?$", pwd)
        if err != nil || !match {
            fmt.Fprint(w, `{"err" : true , "msg" : "密碼必需含有英文字母"}`)
            return
        }

        err = l.NewAcount(id, pwd, name)
        if err == login.ErrorReapeatID{
            fmt.Fprint(w, `{"err" : true , "msg" : "所申請之 ID 重複"}`)
            return
        }else if err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "資料庫連結失敗", "code": 2}`)
            return
        }else{
            fmt.Fprint(w, `{"err" : false}`)
        }

        return
    }else if path == "/function/add_news" {
        // is login?
        loginInfo := login.CheckLogin(w, r)
        if loginInfo == nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "尚未登入", "code" : 1}`)
            return
        }
        user := loginInfo.UserID
        art := article.New();
        // get serial number
        serial, err := art.NewArticle(user)
        if err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "資料庫連結成功但新增文章失敗", "code": 2}`)
            return
        }
        ret := fmt.Sprintf(`{"err" : false, "msg" : %d}`, serial)
        fmt.Fprint(w, ret)
    }else if path == "/function/save_news" || path == "/function/publish_news" || path == "/function/del_news" {
        // is login？
        loginInfo := login.CheckLogin(w, r)
        if loginInfo == nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "尚未登入", "code" : 1}`)
            return
        }

        // write to database
        // step1: fetch http POST
        num, err := strconv.Atoi(function.GET("serial", r))
        if err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "文章代碼錯誤 (POST參數錯誤)", "code": 3}`)
            return
        }
        serial := uint32(num)
        user := loginInfo.UserID
        artType := function.GET("type", r)
        title := function.GET("title", r)
        content := function.GET("content", r)

        art := article.New();
        artFormat := article.Format{
            ID : serial,
            User : user,
            Type : artType,
            Title : title,
            Content : content,
        }

        serverNameList := attachmentJSONtoClientName(function.GET("attachment", r))

        // step3: call Save() or Publish()
        artOperationErr := error(nil)
        if path == "/function/save_news" {
            artOperationErr = art.Save(artFormat, serverNameList)
        }else if path == "/function/publish_news" {
            artOperationErr = art.Publish(artFormat, serverNameList)
        }else if path == "/function/del_news" {
            artOperationErr = art.Del(serial, user)
        }

        if artOperationErr != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "資料庫連結成功但操作文章失敗", "code": 2}`)
            return
        }
        fmt.Fprint(w, `{"err" : false}`)
    }else if path == "/function/get_news"{
        // read news from database
        // step1: read GET
        scope := function.GET("scope", r)
        artType := function.GET("type", r)
        n := function.GET("id", r)
        var serial uint32
        from, to := 0, 19   // Default from = 0, to = 19

        scopes := [...]string{"all", "draft", "published", "public", "public-with-type"}
        checkValidScope := false
        for _, v := range scopes{
            if v == scope{
                checkValidScope = true
                break
            }
        }

        if !checkValidScope{
            if n == ""{
                fmt.Fprint(w, `{"err" : true , "msg" : "錯誤的請求 (GET 參數錯誤)", "code": 3}`)
                return;
            }

            num, err := strconv.Atoi(n)
            if err != nil{
                fmt.Fprint(w, `{"err" : true , "msg" : "文章代碼錯誤 (GET 參數錯誤)", "code": 3}`)
                return
            }
            serial = uint32(num)
        }else{
            if f, scope := function.GET("from", r), function.GET("to", r); f != "" && scope != ""{
                var err error
                from, err = strconv.Atoi(f)
                to, err = strconv.Atoi(scope)
                if err != nil{
                    fmt.Fprint(w, `{"err" : true , "msg" : "from to 代碼錯誤 (GET 參數錯誤)", "code": 3}`)
                    return
                }
            }
        }

        // step2: some request need user id
        user := ""
        if loginInfo := login.CheckLogin(w, r); loginInfo != nil{
            user = loginInfo.UserID
        }

        // step3: connect to database
        art := article.New();

        // step4: call GetLatest(scope, from, to)
        if scope != ""{
            ret := new(struct{
                NewsList []article.Format
                HasNext bool
                Err error
            })
            ret.NewsList, ret.HasNext = art.GetLatest(scope, artType, user, int32(from), int32(to))
            ret.Err = nil;

            // step5: encode to json
            json.NewEncoder(w).Encode(ret)
        }else if n != ""{
            if ret := art.GetArticleBySerial(serial, user); ret != nil{
                json.NewEncoder(w).Encode(ret)
            }else{
                fmt.Fprint(w,`{}`)
            }
        }
    }else if path == "/function/upload"{
        // is login？
        if login.CheckLogin(w, r) == nil{
            fmt.Fprint(w, `{"Err" : true , "Msg" : "尚未登入", "Code" : 1}`)
            return
        }

        r.ParseMultipartForm(32 << 20) // 32MB is the default used by FormFile
        fhs := r.MultipartForm.File["files"]

        ret := []struct{
            Filename string
            Filepath string
        }{}

        for _, fh := range fhs {
            f := files.New()

            if err := f.NewFile(fh); err != nil{
                fmt.Fprint(w, `{"err" : true , "msg" : "新增檔案失敗", "code": 4}`)
                return
            }
            ret = append(ret, struct{
                Filename string
                Filepath string
            }{
                Filename: f.ServerName,
                Filepath: f.Path,
            })
        }
        json.NewEncoder(w).Encode(ret)
    }else if path == "/function/del_attachment"{
        // is login？
        loginInfo := login.CheckLogin(w, r)
        if loginInfo == nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "尚未登入", "code" : 1}`)
            return
        }

        serverName := function.GET("server_name", r)
        serialNum  := function.GET("serial_num", r)
        num, err   := strconv.Atoi(serialNum)
        if err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "文章代碼錯誤 (GET 參數錯誤)", "code": 3}`)
            return
        }

        serverNameList := attachmentJSONtoClientName(function.GET("new_attachment", r))

        // Delete file record in database and delete file in system
        f := files.New()
        if err := f.Del(serverName); err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "檔案資料庫連結失敗或檔案刪除失敗", "code": 2}`)
            return
        }

        // Update databse article (prevent user from not storing the article)
        art := article.New()
        if err := art.UpdateAttachment(uint32(num), serverNameList); err != nil{
            fmt.Fprint(w, `{"err" : true , "msg" : "Article 資料庫更新失敗", "code": 2}`)
            return
        }

        fmt.Fprint(w, `{"err" : false}`)
    }else{
        fmt.Println("未預期的路徑" + path)
        http.Redirect(w, r, "/error/404", 302)
    }
}
