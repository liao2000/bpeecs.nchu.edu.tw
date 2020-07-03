package function
import(
    "database/sql"
    "fmt"
    "time"
    _"github.com/mattn/go-sqlite3"
)

type Article struct{
    db *sql.DB
    artList []Article_Format
    err error
}

type Article_Format struct{
    Id uint32
    User string
    Create_time uint64
    Publish_time uint64
    Last_modified uint64
    Title string
    Content string
}

type ArticleCtrl interface{
    // connect to detabase
    Connect(path string) *sql.DB
    // Create a new article and return seq_num
    NewArticle(user string) uint32
    // Store an article
    SaveArticle(serial uint32, user string, title string, content string)
    // Publish an article
    PublishArticle(serial uint32, user string, title string, content string)
    // Delete an article
    DelArticle(serial uint32, user string)
    // Get the latest article
    GetLatestArticle(whatType string, user string, from int32, to int32) []Article_Format
    // Get article by serial number
    GetArticleBySerial(serial uint32, user string) *Article_Format
    // Get private variables
    GetErr() error
    GetArtList() []Article_Format

    // Generate serial number
    serialNumber(user string) uint32

    // Error process
    errProcess(err error)
}

func (a *Article) GetErr() error{
    return a.err
}

func (a *Article) GetArtList() []Article_Format{
    return a.artList
}

func (a *Article) errProcess(err error){
    if err!=nil{
        fmt.Println(err)
        a.err = err
        return
    }
}

func (a *Article) Connect(path string) *sql.DB{
    db, err := sql.Open("sqlite3", path)
    a.db = db
    a.errProcess(err)
    return a.db
}

func (a *Article) NewArticle(user string) uint32{
    stmt, err := a.db.Prepare("INSERT INTO article(id, user, create_time, publish_time, last_modified, title, content) values(?, ?, ?, ?, ?, ?, ?)")
    a.errProcess(err)
    now := time.Now().Unix()
    serial_num := a.serialNumber(user);
    stmt.Exec(serial_num, user, now, 0, 0, "", "")

    return serial_num
}

func (a *Article) SaveArticle(id uint32, user string, title string, content string){
    stmt, err := a.db.Prepare("UPDATE article SET title=?, content=?, last_modified=?  WHERE id=?")
    a.errProcess(err)
    now := time.Now().Unix()
    _, err = stmt.Exec(title, content, now, id)
    a.errProcess(err)
}

func (a *Article) PublishArticle(id uint32, user string, title string, content string){
    stmt, err := a.db.Prepare("UPDATE article SET title=?, content=?, publish_time=?, last_modified=?  WHERE id=?")
    a.errProcess(err)
    now := time.Now().Unix()
    _, err = stmt.Exec(title, content, now, now, id)
    a.errProcess(err)
}

func (a *Article) DelArticle(serial uint32, user string){
    stmt, err := a.db.Prepare("DELETE from article WHERE id=? and user=?")
    a.errProcess(err)
    _, err = stmt.Exec(serial, user)
    a.errProcess(err)
}

func (a *Article) GetLatestArticle(whatType string, user string, from int32, to int32) (list []Article_Format, hasNext bool){
    var db_query_str string

    switch whatType {
    case "all":
        db_query_str = "SELECT * FROM article WHERE publish_time <> 0 or user = ? ORDER BY `last_modified` DESC, `create_time` DESC, `publish_time` DESC"
    case "draft":
        db_query_str = "SELECT * FROM article WHERE publish_time = 0 and user = ? ORDER BY `last_modified` DESC, `create_time` DESC"
    case "public":
        db_query_str = "SELECT * FROM article WHERE publish_time <> 0 ORDER BY `publish_time` DESC"
    default:
        return nil, false
    }

    // query more than one to decide [hasNext]
    db_query_str += fmt.Sprintf(" limit %d, %d", from, to-from+2)

    var rows *sql.Rows
    var err error
    if whatType == "draft" || whatType == "all" {
        rows, err = a.db.Query(db_query_str, user)
    }else{
        rows, err = a.db.Query(db_query_str)
    }
    a.errProcess(err)

    defer rows.Close()
    for i:= int32(0); rows.Next()  ; i++ {
        var r Article_Format
        err = rows.Scan(&r.Id, &r.User, &r.Create_time, &r.Publish_time, &r.Last_modified, &r.Title, &r.Content)
        a.errProcess(err)
        if i == to-from+1 {
            hasNext = true;
        }else{
            list = append(list, r)
        }
    }

    a.artList = list

    return list, hasNext
}

func (a *Article) GetArticleBySerial(serial uint32, user string) *Article_Format{
    row := a.db.QueryRow("SELECT * FROM article WHERE `id` = ?", serial)

    r := new(Article_Format)
    err := row.Scan(&r.Id, &r.User, &r.Create_time, &r.Publish_time, &r.Last_modified, &r.Title, &r.Content)

    if err == sql.ErrNoRows{
        return nil
    }
    a.errProcess(err)

    // The news has not been published
    if r.Publish_time == 0{
        if r.User != user {
            // Permission denied
            return nil
        }
    }

    return r
}

func (a *Article) serialNumber(user string) uint32{
    rows := a.db.QueryRow("SELECT `id`, `title`, `content` FROM article WHERE `user` = ? ORDER BY `id` DESC", user)

    var num uint32
    var title, content string
    err := rows.Scan(&num, &title, &content)
    a.errProcess(err)

    /* 如果流水號最大的那個消息是空消息則刪除該消息，並回傳該消息序號 */
    if title == "" && content == "" {
        stmt, err := a.db.Prepare("DELETE FROM article WHERE `id` = ?")
        a.errProcess(err)
        _, err = stmt.Exec(num)
        a.errProcess(err)
        return num
    }
    return num + 1
}