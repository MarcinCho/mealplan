package httpapi_test

import "testing"

type memberJSON struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// inviteMember creates a member through the real invite flow and returns their client.
func inviteMember(t *testing.T, e *env, admin *client, login string) (*client, string) {
	t.Helper()
	rec := admin.do("POST", "/api/v1/auth/invites", map[string]string{"role": "member"})
	var inv struct{ Path string }
	decode(t, rec, &inv)
	token := inv.Path[len("/invite/"):]
	m := e.client(t)
	rec = m.do("POST", "/api/v1/auth/invitations/"+token+"/accept", map[string]string{"login": login, "password": pw, "displayName": login})
	if rec.Code != 204 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	m.useCSRF()
	return m, meID(t, m)
}

func TestChangeRoleAndRemoveMember(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)
	adminID := meID(t, admin)
	member, memberID := inviteMember(t, e, admin, "mia")

	if rec := member.do("PATCH", "/api/v1/household/members/"+memberID, map[string]string{"role": "admin"}); rec.Code != 403 {
		t.Fatalf("member must not change roles: %d", rec.Code)
	}
	if rec := member.do("DELETE", "/api/v1/household/members/"+adminID, nil); rec.Code != 403 {
		t.Fatalf("member must not remove: %d", rec.Code)
	}
	if rec := admin.do("PATCH", "/api/v1/household/members/"+memberID, map[string]string{"role": "bogus"}); rec.Code != 422 {
		t.Fatalf("bad role: %d", rec.Code)
	}
	if rec := admin.do("PATCH", "/api/v1/household/members/"+memberID, map[string]string{"role": "admin"}); rec.Code != 204 {
		t.Fatalf("promote: %d %s", rec.Code, rec.Body)
	}
	if rec := admin.do("PATCH", "/api/v1/household/members/"+adminID, map[string]string{"role": "member"}); rec.Code != 204 {
		t.Fatalf("demote self while another admin exists: %d", rec.Code)
	}
	// admin is now an ordinary member; mia is the only admin.
	if rec := member.do("PATCH", "/api/v1/household/members/"+memberID, map[string]string{"role": "member"}); rec.Code != 409 || errCode(t, rec) != "last_admin" {
		t.Fatalf("last admin demotion: %d %s", rec.Code, rec.Body)
	}
	if rec := member.do("DELETE", "/api/v1/household/members/"+memberID, nil); rec.Code != 409 || errCode(t, rec) != "last_admin" {
		t.Fatalf("last admin removal: %d %s", rec.Code, rec.Body)
	}
	if rec := member.do("DELETE", "/api/v1/household/members/"+adminID, nil); rec.Code != 204 {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	if rec := admin.do("GET", "/api/v1/auth/me", nil); rec.Code != 401 {
		t.Fatalf("removed member session must be dead: %d", rec.Code)
	}
}

func TestRemovedMemberCannotLogInAndDoesNotRevealAccount(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)
	_, memberID := inviteMember(t, e, admin, "mia")
	if rec := admin.do("DELETE", "/api/v1/household/members/"+memberID, nil); rec.Code != 204 {
		t.Fatalf("remove: %d", rec.Code)
	}
	anon := e.client(t)
	rec := anon.do("POST", "/api/v1/auth/login", map[string]string{"login": "mia", "password": pw})
	if rec.Code != 401 || errCode(t, rec) != "invalid_credentials" {
		t.Fatalf("removed member login must look like bad credentials: %d %s", rec.Code, rec.Body)
	}
	ghost := anon.do("POST", "/api/v1/auth/login", map[string]string{"login": "nobody", "password": pw})
	if ghost.Code != rec.Code || ghost.Body.String() != rec.Body.String() {
		t.Fatalf("responses differ: %q vs %q", ghost.Body, rec.Body)
	}
}

func TestRemovingCookClearsPlanEntries(t *testing.T) {
	e := newEnv(t)
	admin := e.client(t)
	setup(t, admin)
	_, memberID := inviteMember(t, e, admin, "mia")
	rec := admin.do("POST", "/api/v1/plan/2026-11-02/entries", map[string]any{"date": "2026-11-03", "slot": "dinner", "freeText": "Pizza", "cookUserId": memberID})
	if rec.Code != 201 {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	admin.do("DELETE", "/api/v1/household/members/"+memberID, nil)
	var wk weekJSON
	decode(t, admin.do("GET", "/api/v1/plan/2026-11-02", nil), &wk)
	if len(wk.Entries) != 1 || wk.Entries[0].CookUserID != nil {
		t.Fatalf("cook must be cleared, entry kept: %+v", wk)
	}
}

func TestMemberRoutesDoNotTouchOtherHouseholds(t *testing.T) {
	e := newEnv(t)
	a := e.client(t)
	setup(t, a)
	b := e.secondHouseholdClient(t)
	bID := meID(t, b)
	if rec := a.do("DELETE", "/api/v1/household/members/"+bID, nil); rec.Code != 404 {
		t.Fatalf("remove foreign: %d", rec.Code)
	}
	if rec := a.do("PATCH", "/api/v1/household/members/"+bID, map[string]string{"role": "member"}); rec.Code != 404 {
		t.Fatalf("patch foreign: %d", rec.Code)
	}
	if rec := b.do("GET", "/api/v1/auth/me", nil); rec.Code != 200 {
		t.Fatalf("victim damaged: %d", rec.Code)
	}
}
