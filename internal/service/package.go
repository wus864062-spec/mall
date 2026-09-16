package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mall/internal/biz"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const (
	OperationAdminHTTPListPackages  = "/api.admin.v1.AdminService/HTTPListPackages"
	OperationAdminHTTPCreatePackage = "/api.admin.v1.AdminService/HTTPCreatePackage"
	OperationAdminHTTPUpdatePackage = "/api.admin.v1.AdminService/HTTPUpdatePackage"
	OperationAdminHTTPPackageStatus = "/api.admin.v1.AdminService/HTTPPackageStatus"
	OperationAdminHTTPPackageImage  = "/api.admin.v1.AdminService/HTTPPackageImage"
)

func (s *AdminService) UploadsDir() string {
	if s == nil || s.uc == nil {
		return "data/uploads"
	}
	return s.uc.UploadsDir()
}

func packageJSON(p *biz.Package) map[string]interface{} {
	if p == nil {
		return nil
	}
	status := "0"
	if p.Status == biz.ProductOnSale {
		status = "1"
	}
	desc := p.Description
	return map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"description": desc,
		"one":         desc,
		"three":       desc,
		"amount":      biz.FenToUstd(p.AmountFen),
		"amountFen":   p.AmountFen,
		"image":       p.Image,
		"days":        p.ReleaseDays,
		"releaseDays": p.ReleaseDays,
		"status":      status,
		"productId":   p.ProductID,
		"sortOrder":   p.SortOrder,
	}
}

func (s *AdminService) HTTPListPackages(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPListPackages)
	raw := ctx.Request().URL.Query().Get("days")
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		days, err := biz.ParseReleaseDaysQuery(raw)
		if err != nil {
			return nil, err
		}
		list, err := s.uc.ListPackages(c, days)
		if err != nil {
			return nil, err
		}
		st, err := s.uc.GetSettings(c)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]interface{}, 0, len(list))
		for _, p := range list {
			row := packageJSON(p)
			if row != nil && p != nil {
				row["pairCapFen"] = biz.PairCapFenWith(p.AmountFen, st.PairCaps)
			}
			rows = append(rows, row)
		}
		return rows, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	rows := out.([]map[string]interface{})
	return ctx.Result(200, map[string]interface{}{"goods": rows, "count": len(rows)})
}

func (s *AdminService) HTTPCreatePackage(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPCreatePackage)
	body, err := readPackageBody(ctx)
	if err != nil {
		return biz.ErrInvalidArgument
	}
	days, err := packageDaysFrom(body.Days, ctx.Request().URL.Query().Get("days"))
	if err != nil {
		return err
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.CreatePackage(c, &biz.Package{
			Name:        body.Name,
			Description: body.Description,
			AmountFen:   packageAmountFen(body),
			ReleaseDays: days,
			Image:       derefString(body.Image),
			Status:      packageStatus(body.Status, true),
		})
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	row := packageJSON(out.(*biz.Package))
	return ctx.Result(200, map[string]interface{}{"goods": row, "package": row})
}

func (s *AdminService) HTTPUpdatePackage(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPUpdatePackage)
	id, _ := strconv.ParseInt(ctx.Vars().Get("id"), 10, 64)
	body, err := readPackageBody(ctx)
	if err != nil {
		return biz.ErrInvalidArgument
	}
	days, err := packageDaysFrom(body.Days, ctx.Request().URL.Query().Get("days"))
	if err != nil {
		return err
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.UpdatePackage(c, &biz.Package{
			ID:          id,
			Name:        body.Name,
			Description: body.Description,
			AmountFen:   packageAmountFen(body),
			ReleaseDays: days,
			Image:       derefString(body.Image),
			Status:      packageStatus(body.Status, false),
		}, body.Image != nil)
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	row := packageJSON(out.(*biz.Package))
	return ctx.Result(200, map[string]interface{}{"goods": row, "package": row})
}

func (s *AdminService) HTTPSetPackageStatus(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPPackageStatus)
	id, _ := strconv.ParseInt(ctx.Vars().Get("id"), 10, 64)
	body, err := readPackageBody(ctx)
	if err != nil {
		return biz.ErrInvalidArgument
	}
	days, err := packageDaysFrom(body.Days, ctx.Request().URL.Query().Get("days"))
	if err != nil {
		return err
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		return s.uc.SetPackageStatus(c, id, days, packageStatus(body.Status, false))
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	row := packageJSON(out.(*biz.Package))
	return ctx.Result(200, map[string]interface{}{"goods": row, "package": row})
}

func (s *AdminService) HTTPUploadPackageImage(ctx khttp.Context) error {
	khttp.SetOperation(ctx, OperationAdminHTTPPackageImage)
	r := ctx.Request()
	if err := r.ParseMultipartForm(int64(biz.MaxPackageImageBytes) + 1<<20); err != nil {
		return biz.ErrImageUpload
	}
	days, err := biz.ParseReleaseDaysQuery(r.FormValue("days"))
	if err != nil {
		return err
	}
	if days == 0 {
		days, err = biz.ParseReleaseDaysQuery(r.URL.Query().Get("days"))
		if err != nil {
			return err
		}
	}
	if days == 0 {
		return biz.ErrReleaseDays
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return biz.ErrImageRequired
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(biz.MaxPackageImageBytes)+1))
	if err != nil {
		return biz.ErrImageUpload
	}
	ext, err := biz.PackageImageExt(raw)
	if err != nil {
		return err
	}
	h := ctx.Middleware(func(c context.Context, _ interface{}) (interface{}, error) {
		if _, err := s.uc.RequireAdmin(c); err != nil {
			return nil, err
		}
		url, err := s.writePackageImage(r, days, ext, raw)
		if err != nil {
			return nil, err
		}
		return url, nil
	})
	out, err := h(ctx, nil)
	if err != nil {
		return err
	}
	return ctx.Result(200, map[string]interface{}{"status": "ok", "url": out.(string)})
}

func (s *AdminService) writePackageImage(r *http.Request, days int64, ext string, raw []byte) (string, error) {
	dir := filepath.Join(s.UploadsDir(), "packages", strconv.FormatInt(days, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", biz.ErrImageUpload
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", biz.ErrImageUpload
	}
	name := hex.EncodeToString(b[:]) + ext
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		return "", biz.ErrImageUpload
	}
	rel := "/uploads/packages/" + strconv.FormatInt(days, 10) + "/" + name
	return publicBaseURL(r) + rel, nil
}

type packageBody struct {
	Days        interface{} `json:"days"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	AmountFen   interface{} `json:"amount_fen"`
	Amount      interface{} `json:"amount"`
	Image       *string     `json:"image"`
	Status      interface{} `json:"status"`
}

func readPackageBody(ctx khttp.Context) (packageBody, error) {
	var body packageBody
	if err := readJSON(ctx, &body); err != nil {
		return body, err
	}
	return body, nil
}

func packageDaysFrom(v interface{}, query string) (int64, error) {
	if n := parseInt64(v); n != 0 {
		if !biz.ValidReleaseDays(n) {
			return 0, biz.ErrReleaseDays
		}
		return n, nil
	}
	return biz.ParseReleaseDaysQuery(query)
}

func packageAmountFen(body packageBody) int64 {
	// 标价用 amount_fen；amount 是 USDT 展示值，截断进 fen。都不是 EffectivePackageFen。
	if n := parseInt64(body.AmountFen); n > 0 {
		return n
	}
	return parsePriceFen(body.Amount)
}

func packageStatus(v interface{}, create bool) int32 {
	if v == nil {
		if create {
			return biz.ProductOnSale
		}
		return 0
	}
	if parseInt64(v) == 0 {
		return 0
	}
	return biz.ProductOnSale
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func publicBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
