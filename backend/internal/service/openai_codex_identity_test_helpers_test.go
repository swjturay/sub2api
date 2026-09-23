//go:build unit

package service

func codexShadowTestRows(convergence bool) (parent, shadow, other *Account) {
	parent = wireProfileTestAccount(convergence)
	parent.Credentials["chatgpt_user_id"] = "offline-user"
	parentID := parent.ID
	shadow = wireProfileTestAccount(convergence)
	shadow.ID = parentID + 3
	shadow.ParentAccountID = &parentID
	shadow.Credentials = map[string]any{"model_mapping": map[string]any{}}
	other = wireProfileTestAccount(convergence)
	other.ID = parentID + 9
	other.Credentials = map[string]any{"access_token": "offline-token-b", "chatgpt_account_id": "other-account"}
	return parent, shadow, other
}
