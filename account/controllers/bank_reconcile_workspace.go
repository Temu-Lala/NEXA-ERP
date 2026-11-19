package controllers

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"nexa/core/orm"
	"nexa/core/server/router"
	banksvc "nexa_addons/account/services"
)

func init() {
	router.Register(http.MethodGet, "/account/bank/reconcile", router.AuthSession, reconWorkspaceHandler)
	router.Register(http.MethodPost, "/account/bank/reconcile/match", router.AuthSession, reconMatchHandler)
	router.Register(http.MethodPost, "/account/bank/reconcile/match_line", router.AuthSession, reconMatchLineHandler)
	router.Register(http.MethodPost, "/account/bank/reconcile/import", router.AuthSession, reconImportHandler)
}

var reconTmpl = template.Must(template.New("recon").Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Bank Reconciliation</title>
<style>
:root{--bg:#f6f7f9;--card:#fff;--line:#e3e6ea;--ink:#1f2937;--muted:#6b7280;--accent:#0b62d6;--ok:#15803d;--warn:#b45309}
*{box-sizing:border-box}
body{font-family:-apple-system,"Segoe UI",Roboto,sans-serif;margin:0;background:var(--bg);color:var(--ink)}
header{background:#fff;border-bottom:1px solid var(--line);padding:14px 24px;display:flex;align-items:center;gap:16px;flex-wrap:wrap}
header h1{font-size:18px;margin:0;font-weight:650}
header .spacer{flex:1}
.import{display:flex;gap:8px;align-items:center;background:#f1f5f9;border:1px solid var(--line);border-radius:8px;padding:8px 12px}
.import input[type=number]{width:90px;padding:6px 8px;border:1px solid var(--line);border-radius:6px}
.import input[type=file]{font-size:12px;max-width:220px}
.import button,.btn{background:var(--accent);color:#fff;border:none;border-radius:6px;padding:7px 14px;font-size:13px;font-weight:600;cursor:pointer}
.btn.ghost{background:#fff;color:var(--ink);border:1px solid var(--line)}
.btn.small{padding:4px 10px;font-size:12px}
main{padding:20px 24px;display:grid;grid-template-columns:minmax(0,3fr) minmax(0,2fr);gap:20px;align-items:start}
@media(max-width:1100px){main{grid-template-columns:1fr}}
.card{background:var(--card);border:1px solid var(--line);border-radius:10px;overflow:hidden}
.card h2{font-size:14px;margin:0;padding:12px 16px;border-bottom:1px solid var(--line);font-weight:650;display:flex;justify-content:space-between;align-items:center}
.card h2 .count{font-size:12px;color:var(--muted);font-weight:500}
table{width:100%;border-collapse:collapse;font-size:13px}
th{font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:var(--muted);text-align:left;padding:8px 14px;border-bottom:1px solid var(--line);background:#fafbfc}
td{padding:9px 14px;border-bottom:1px solid #f0f2f4;vertical-align:top}
tr:last-child td{border-bottom:none}
.num{text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
.pos{color:var(--ok)} .neg{color:#b91c1c}
.badge{display:inline-flex;align-items:center;gap:5px;font-size:11px;font-weight:600;padding:2px 9px;border-radius:999px}
.badge.ok{background:#dcfce7;color:var(--ok)}
.badge.open{background:#fef3c7;color:var(--warn)}
.stmt-name{font-weight:550}
.stmt-sub{font-size:12px;color:var(--muted)}
.cand{display:flex;align-items:center;gap:10px;padding:8px 14px;border-bottom:1px solid #f0f2f4}
.cand:last-child{border-bottom:none}
.cand .grow{flex:1;min-width:0}
.cand .lbl{font-size:13px;font-weight:550;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.cand .sub{font-size:12px;color:var(--muted)}
.cand .diff{font-size:11px;color:var(--muted);font-variant-numeric:tabular-nums;white-space:nowrap}
.empty{padding:24px 16px;text-align:center;color:var(--muted);font-size:13px}
.flash{background:#eff6ff;border:1px solid #bfdbfe;color:#1e40af;padding:10px 24px;font-size:13px}
.flash.err{background:#fef2f2;border-color:#fecaca;color:#991b1b}
form.inline{display:inline}
.hint{font-size:12px;color:var(--muted);padding:0 24px 16px}
</style></head><body>
<header>
  <h1>Bank Reconciliation</h1>
  <span class="spacer"></span>
  <form class="import" method="post" action="/account/bank/reconcile/import" enctype="multipart/form-data">
    <label style="font-size:12px;color:var(--muted)">Statement
      <input type="number" name="statement_id" required placeholder="ID"/>
    </label>
    <input type="file" name="file" accept=".csv,text/csv" required/>
    <button type="submit">Import CSV</button>
  </form>
</header>
{{if .Flash}}<div class="flash{{if .FlashErr}} err{{end}}">{{.Flash}}</div>{{end}}
<main>
  <section class="card">
    <h2>Statement Lines <span class="count">{{len .Lines}} total · {{.OpenCount}} open</span></h2>
    <table>
      <tr><th>Date</th><th>Description</th><th>Partner</th><th class="num">Amount</th><th>Status</th><th></th></tr>
      {{range .Lines}}<tr>
        <td style="white-space:nowrap">{{.Date}}</td>
        <td><div class="stmt-name">{{.Name}}</div>{{if .Statement}}<div class="stmt-sub">{{.Statement}}</div>{{end}}</td>
        <td>{{.Partner}}</td>
        <td class="num {{if .Negative}}neg{{else}}pos{{end}}">{{.Amount}}</td>
        <td>{{if .Reconciled}}<span class="badge ok">Reconciled</span>{{else}}<span class="badge open">Open</span>{{end}}</td>
        <td style="white-space:nowrap">
          {{if not .Reconciled}}
          <form class="inline" method="post" action="/account/bank/reconcile/match"><input type="hidden" name="line_id" value="{{.ID}}"/><button class="btn small">Auto-match</button></form>
          {{end}}
        </td>
      </tr>
      {{if and (not .Reconciled) .Suggestions}}
      <tr><td colspan="6" style="background:#fafbfc;padding:0">
        <div style="padding:6px 14px 10px 40px">
          {{range .Suggestions}}<div class="cand">
            <div class="grow">
              <div class="lbl">{{.Label}}</div>
              <div class="sub">{{.Partner}}{{if .MoveName}} · {{.MoveName}}{{end}}</div>
            </div>
            <span class="num" style="font-weight:600">{{printf "%.2f" .Amount}}</span>
            <span class="diff">Δ {{printf "%.2f" .Diff}}</span>
            <form class="inline" method="post" action="/account/bank/reconcile/match_line">
              <input type="hidden" name="line_id" value="{{.LineID}}"/>
              <input type="hidden" name="move_line_id" value="{{.MoveLineID}}"/>
              <button class="btn small ghost">Match</button>
            </form>
          </div>{{end}}
        </div>
      </td></tr>
      {{end}}{{end}}
    </table>
    {{if eq (len .Lines) 0}}<div class="empty">No statement lines yet — import a CSV or create statement lines first.</div>{{end}}
  </section>
  <section class="card">
    <h2>How it works</h2>
    <table><tbody>
      <tr><td style="width:130px"><b>Auto-match</b></td><td>Matches each open line to the best journal item using reconciliation models, then amount heuristics.</td></tr>
      <tr><td><b>Suggestions</b></td><td>Ranked journal-item candidates appear under each open line — pick the right one and press <b>Match</b>.</td></tr>
      <tr><td><b>CSV import</b></td><td>Columns: <code>date,label,amount</code>. Import into an existing bank statement by ID.</td></tr>
    </tbody></table>
    <p class="hint" style="padding:12px 16px">Matched amounts update the journal item's residual and the invoice payment status automatically.</p>
  </section>
</main>
</body></html>`))

type reconLine struct {
	ID, Date, Name, Amount, Partner, Statement string
	Negative, Reconciled                       bool
	Suggestions                                []reconSuggestion
}

type reconSuggestion struct {
	LineID, MoveLineID             int64
	Label, Partner, MoveName       string
	Amount, Diff                   float64
}

func reconWorkspaceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := orm.ContextWithBypass(r.Context(), true)
	lines, _ := orm.Search(ctx, "account.bank.statement.line", nil)
	view := make([]reconLine, 0, len(lines))
	openCount := 0
	for _, ln := range lines {
		id, _ := orm.CoerceInt64(ln["id"])
		rec := asBool(ln["is_reconciled"])
		if !rec {
			openCount++
		}
		partner := ""
		if pid, ok := orm.CoerceInt64(ln["partner_id"]); ok && pid > 0 {
			if p, err := orm.SearchOne(ctx, "core.partner", map[string]interface{}{"id": pid}); err == nil {
				partner = orm.AsString(p["name"])
			}
		}
		statement := ""
		if sid, ok := orm.CoerceInt64(ln["statement_id"]); ok && sid > 0 {
			if s, err := orm.SearchOne(ctx, "account.bank.statement", map[string]interface{}{"id": sid}); err == nil {
				statement = orm.AsString(s["name"])
			}
		}
		amount := numericFloat(ln["amount"])
		rl := reconLine{
			ID:         strconv.Itoa(int(id)),
			Date:       orm.AsString(ln["date"]),
			Name:       orm.AsString(ln["name"]),
			Amount:     formatReconAmount(amount),
			Partner:    partner,
			Statement:  statement,
			Negative:   amount < 0,
			Reconciled: rec,
		}
		if !rec && id > 0 {
			if matches, err := banksvc.SuggestedMatches(ctx, int(id)); err == nil {
				for _, m := range matches {
					moveName := ""
					if ml, err := orm.SearchOne(ctx, "account.move.line", map[string]interface{}{"id": m.MoveLineID}); err == nil {
						if mid, ok := orm.CoerceInt64(ml["move_id"]); ok && mid > 0 {
							if mv, err := orm.SearchOne(ctx, "account.move", map[string]interface{}{"id": mid}); err == nil {
								moveName = orm.AsString(mv["name"])
							}
						}
					}
					rl.Suggestions = append(rl.Suggestions, reconSuggestion{
						LineID:     id,
						MoveLineID: m.MoveLineID,
						Label:      m.Label,
						Partner:    m.Partner,
						MoveName:   moveName,
						Amount:     m.Amount,
						Diff:       m.Diff,
					})
					if len(rl.Suggestions) >= 5 {
						break
					}
				}
			}
		}
		view = append(view, rl)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = reconTmpl.Execute(w, map[string]interface{}{
		"Lines":     view,
		"OpenCount": openCount,
		"Flash":     r.URL.Query().Get("msg"),
		"FlashErr":  r.URL.Query().Get("err") == "1",
	})
}

func reconMatchHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("line_id")))
	if id <= 0 {
		http.Error(w, "missing line", http.StatusBadRequest)
		return
	}
	ctx := orm.ContextWithBypass(r.Context(), true)
	if _, err := banksvc.MatchStatementLine(ctx, id); err != nil {
		http.Redirect(w, r, "/account/bank/reconcile?err=1&msg="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/account/bank/reconcile?msg="+url.QueryEscape("Line auto-matched"), http.StatusSeeOther)
}

// reconMatchLineHandler applies an explicit user-chosen journal item to a
// statement line — the counterpart to auto-match for ambiguous cases.
func reconMatchLineHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	lineID, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("line_id")))
	moveLineID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("move_line_id")), 10, 64)
	if lineID <= 0 || moveLineID <= 0 {
		http.Error(w, "missing line or match", http.StatusBadRequest)
		return
	}
	ctx := orm.ContextWithBypass(r.Context(), true)
	line, err := orm.SearchOne(ctx, "account.bank.statement.line", map[string]interface{}{"id": lineID})
	if err != nil {
		http.Error(w, "statement line not found", http.StatusBadRequest)
		return
	}
	if asBool(line["is_reconciled"]) {
		http.Redirect(w, r, "/account/bank/reconcile?err=1&msg="+url.QueryEscape("Line already reconciled"), http.StatusSeeOther)
		return
	}
	amount := numericFloat(line["amount"])
	if err := banksvc.ReconcileStatementLine(ctx, int(moveLineID), amount); err != nil {
		http.Redirect(w, r, "/account/bank/reconcile?err=1&msg="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if err := orm.UpdateRecordByID(ctx, "account.bank.statement.line", lineID, map[string]interface{}{
		"is_reconciled": true,
		"move_line_id":  moveLineID,
	}); err != nil {
		http.Redirect(w, r, "/account/bank/reconcile?err=1&msg="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/account/bank/reconcile?msg="+url.QueryEscape("Matched to journal item"), http.StatusSeeOther)
}

func reconImportHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	stmtID, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("statement_id")))
	if stmtID <= 0 {
		http.Error(w, "statement_id required", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	reader := csv.NewReader(io.LimitReader(file, 2<<20))
	rows, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "invalid csv", http.StatusBadRequest)
		return
	}
	if len(rows) > 0 && looksLikeCSVHeader(rows[0]) {
		rows = rows[1:]
	}
	ctx := orm.ContextWithBypass(r.Context(), true)
	if err := banksvc.ImportCSVStatement(ctx, stmtID, rows); err != nil {
		http.Redirect(w, r, "/account/bank/reconcile?err=1&msg="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/account/bank/reconcile?msg="+url.QueryEscape(fmt.Sprintf("Imported %d lines", len(rows))), http.StatusSeeOther)
}

func looksLikeCSVHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(row[0]))
	return first == "date" || first == "label"
}

func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	default:
		return false
	}
}

func numericFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func formatReconAmount(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}
