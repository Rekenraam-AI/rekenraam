package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #291. A category is an income or expense account carrying category metadata,
// and the category API owns its rules: protected built-ins, sibling names,
// immutable type, genesis opening date. The generic account API must not be a
// second way in. Plain income and expense accounts without category metadata
// (dividend income, withholding tax, borrow costs) remain account-managed.

func TestAccountAPIRefusesToManageCategories(t *testing.T) {
	t.Parallel()

	handler, _ := newSetupTestHandler(t)
	cookie, csrf, usd := setupAccountAPITest(t, handler)
	setup := mutateCategoriesSetup(t, handler, cookie, csrf, http.StatusCreated)
	other := categoryByCode(t, setup.Categories, "expense_other")
	checking := createLedgerAccount(t, handler, cookie, csrf, "Checking", "asset", "checking", usd, 2)
	salary := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Salary","category_type":"income"}`)
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-06-07",
		posting(checking.ID, 10000, 2, usd), posting(salary.ID, -10000, 2, usd)), http.StatusCreated)

	for _, category := range []struct {
		id                  int64
		accountClass        string
		name, wantName, key string
	}{
		{id: other.ID, accountClass: "expense", key: "expense.other"},
		{id: salary.ID, accountClass: "income", wantName: "Salary"},
	} {
		path := "/api/v1/accounts/" + strconvFormatInt(category.id)
		for _, refusal := range []struct{ method, suffix, body string }{
			{http.MethodPatch, "", `{"name":"Renamed","account_class":"` + category.accountClass + `","account_kind":"` + category.accountClass + `"}`},
			{http.MethodPost, "/close", `{}`},
			{http.MethodPost, "/archive", `{}`},
			{http.MethodPost, "/reopen", `{}`},
			{http.MethodPost, "/restore", `{}`},
			{http.MethodDelete, "", ""},
		} {
			res := referenceDataRequest(handler, cookie, csrf, refusal.method, path+refusal.suffix, refusal.body)
			require.Equalf(t, http.StatusConflict, res.Code, "%s %s%s: %s", refusal.method, path, refusal.suffix, res.Body.String())
			assert.Equal(t, "CONFLICT", apiErrorCode(t, res))
		}

		// The category is untouched and still reachable through its own API.
		read := referenceDataRequest(handler, cookie, csrf, http.MethodGet, "/api/v1/categories/"+strconvFormatInt(category.id), "")
		require.Equalf(t, http.StatusOK, read.Code, read.Body.String())
		assert.Contains(t, read.Body.String(), `"status":"active"`)
		if category.key != "" {
			assert.Contains(t, read.Body.String(), `"builtin_key":"`+category.key+`"`)
		} else {
			assert.Contains(t, read.Body.String(), `"name":"`+category.wantName+`"`)
		}
	}

	// Nor can the account API create a category: not under a category parent,
	// and not by writing category metadata itself.
	for _, body := range []string{
		`{"name":"Bonus","account_class":"income","account_kind":"income","parent_account_id":` + strconvFormatInt(salary.ID) + `}`,
		`{"name":"Forged","account_class":"expense","account_kind":"expense","metadata":{"category":{"type":"expense","is_builtin":true,"builtin_key":"expense.other"}}}`,
	} {
		res := referenceDataRequest(handler, cookie, csrf, http.MethodPost, "/api/v1/accounts", body)
		require.Equalf(t, http.StatusBadRequest, res.Code, res.Body.String())
		assert.Equal(t, "VALIDATION_FAILED", apiErrorCode(t, res))
	}
}

func TestAccountAPIStillManagesPlainIncomeAndExpenseAccounts(t *testing.T) {
	t.Parallel()

	handler, _ := newSetupTestHandler(t)
	cookie, csrf, usd := setupAccountAPITest(t, handler)
	salary := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Salary","category_type":"income"}`)
	dividends := createLedgerAccount(t, handler, cookie, csrf, "Dividend income", "income", "income", usd, 2)
	path := "/api/v1/accounts/" + strconvFormatInt(dividends.ID)

	renamed := patchAccount(t, handler, cookie, csrf, dividends.ID, `{
		"name":"Dividends","account_class":"income","account_kind":"income",
		"default_commodity_id":`+strconvFormatInt(usd)+`,"quantity_scale_override":2
	}`, http.StatusOK)
	assert.Equal(t, "Dividends", renamed.Name)

	// Re-parenting it under a category would turn it into a category's child
	// outside the category rules.
	res := referenceDataRequest(handler, cookie, csrf, http.MethodPatch, path, `{
		"name":"Dividends","account_class":"income","account_kind":"income",
		"default_commodity_id":`+strconvFormatInt(usd)+`,"quantity_scale_override":2,
		"parent_account_id":`+strconvFormatInt(salary.ID)+`
	}`)
	require.Equalf(t, http.StatusBadRequest, res.Code, res.Body.String())
	assert.Equal(t, "VALIDATION_FAILED", apiErrorCode(t, res))

	mutateAccount(t, handler, cookie, csrf, http.MethodPost, path+"/close", http.StatusOK)
	mutateAccount(t, handler, cookie, csrf, http.MethodPost, path+"/archive", http.StatusOK)
	mutateAccountNoResponse(t, handler, cookie, csrf, http.MethodDelete, path, http.StatusNoContent)
}
