package app

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile("^[0-9a-f]{32}$")

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

type Problem struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (p *Problem) Error() string              { return p.Message }
func fail(status int, code, msg string) error { return &Problem{status, code, msg} }

type Ingredient struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}
type CookLog struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	Note   string `json:"note"`
	Rating int    `json:"rating"`
}
type Media struct {
	ID        string `json:"id"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int64  `json:"bytes"`
	CreatedAt string `json:"createdAt"`
}
type Record struct {
	ID           string       `json:"id"`
	Kind         string       `json:"kind"`
	Revision     int          `json:"revision"`
	Name         string       `json:"name"`
	Category     string       `json:"category"`
	Notes        string       `json:"notes"`
	Tags         []string     `json:"tags"`
	Ingredients  []Ingredient `json:"ingredients"`
	Steps        []string     `json:"steps"`
	Minutes      int          `json:"minutes"`
	Servings     int          `json:"servings"`
	Favorite     bool         `json:"favorite"`
	Logs         []CookLog    `json:"logs"`
	PhotoIDs     []string     `json:"photoIds"`
	Quantity     int          `json:"quantity"`
	Unit         string       `json:"unit"`
	Location     string       `json:"location"`
	PurchaseDate string       `json:"purchaseDate"`
	ExpiryDate   string       `json:"expiryDate"`
	CreatedAt    string       `json:"createdAt"`
	UpdatedAt    string       `json:"updatedAt"`
	DeletedAt    *string      `json:"deletedAt"`
}
type WriteInput struct {
	Record
	Action string  `json:"action"`
	Log    CookLog `json:"log"`
}

func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, e := time.Parse("2006-01-02", s)
	return e == nil
}
func (r *Record) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 120 {
		return fail(422, "name", "名称需要填写，最多 120 字")
	}
	if r.Kind != "recipe" && r.Kind != "pantry" {
		return fail(422, "kind", "无效的记录类型")
	}
	if utf8.RuneCountInString(r.Notes) > 10000 || utf8.RuneCountInString(r.Category) > 40 || len(r.Tags) > 20 || len(r.Ingredients) > 100 || len(r.Steps) > 100 || len(r.PhotoIDs) > 10 {
		return fail(422, "limit", "内容过多：最多 10 张照片、100 条食材或步骤")
	}
	if r.Minutes < 0 || r.Minutes > 10080 || r.Servings < 0 || r.Servings > 100 || r.Quantity < 0 || r.Quantity > 9999 {
		return fail(422, "number", "请检查时长、人数或数量")
	}
	for _, s := range []string{r.Location, r.Unit} {
		if utf8.RuneCountInString(s) > 100 {
			return fail(422, "text", "位置或单位过长")
		}
	}
	if !validDate(r.PurchaseDate) || !validDate(r.ExpiryDate) {
		return fail(422, "date", "日期格式应为 年-月-日")
	}
	if r.PurchaseDate != "" && r.ExpiryDate != "" && r.ExpiryDate < r.PurchaseDate {
		return fail(422, "date", "到期日期不能早于购买日期")
	}
	seen := map[string]bool{}
	for _, id := range r.PhotoIDs {
		if !idPattern.MatchString(id) || seen[id] {
			return fail(422, "photos", "照片编号无效或重复")
		}
		seen[id] = true
	}
	ingredients := []Ingredient{}
	for _, i := range r.Ingredients {
		i.Name = strings.TrimSpace(i.Name)
		i.Amount = strings.TrimSpace(i.Amount)
		if i.Name == "" && i.Amount == "" {
			continue
		}
		if i.Name == "" || utf8.RuneCountInString(i.Name) > 100 || utf8.RuneCountInString(i.Amount) > 100 {
			return fail(422, "ingredient", "请填写食材名称，每项最多 100 字")
		}
		ingredients = append(ingredients, i)
	}
	r.Ingredients = ingredients
	steps := []string{}
	for _, s := range r.Steps {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > 4000 {
			return fail(422, "step", "单步做法最多 4000 字")
		}
		steps = append(steps, s)
	}
	r.Steps = steps
	tags := []string{}
	seen = map[string]bool{}
	for _, t := range r.Tags {
		t = strings.TrimSpace(t)
		if utf8.RuneCountInString(t) > 40 {
			return fail(422, "tag", "标签最多 40 字")
		}
		if t != "" && !seen[t] {
			tags = append(tags, t)
			seen[t] = true
		}
	}
	r.Tags = tags
	if r.PhotoIDs == nil {
		r.PhotoIDs = []string{}
	}
	if r.Logs == nil {
		r.Logs = []CookLog{}
	}
	return nil
}
func (l *CookLog) Validate() error {
	if l.Date == "" || !validDate(l.Date) || l.Rating < 0 || l.Rating > 5 || utf8.RuneCountInString(l.Note) > 4000 {
		return fail(422, "log", "请检查下厨日期、评分和心得（最多 4000 字）")
	}
	if l.ID != "" && !idPattern.MatchString(l.ID) {
		return fail(422, "log", "无效的下厨记录编号")
	}
	return nil
}
