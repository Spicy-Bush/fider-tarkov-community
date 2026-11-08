package actions_test

import (
    "encoding/json"
    "testing"

    "github.com/Spicy-Bush/fider-tarkov-community/app/actions"
    "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func TestAuthenticationKeysCannotBeSuppliedByJSON(t *testing.T) {
    inputs := []struct {
        name string
        body string
    }{
        {"ordinary request", `{"email":"member@example.com"}`},
        {"field name", `{"email":"member@example.com","VerificationKey":"chosen-token"}`},
        {"lowercase", `{"email":"member@example.com","verificationkey":"chosen-token"}`},
        {"escaped name", `{"email":"member@example.com","\u0056erificationKey":"chosen-token"}`},
        {"duplicate fields", `{"email":"member@example.com","VerificationKey":"first","verificationkey":"second"}`},
        {"null", `{"email":"member@example.com","VerificationKey":null}`},
        {"wrong type", `{"email":"member@example.com","VerificationKey":{"value":"chosen-token"}}`},
    }

    for _, input := range inputs {
        t.Run(input.name, func(t *testing.T) {
            signIn := actions.NewSignInByEmail()
            signUp := actions.NewCreateTenant()
            changeEmail := actions.NewChangeUserEmail()

            targets := []struct {
                name string
                action any
                key *string
            }{
                {"sign in", signIn, &signIn.VerificationKey},
                {"sign up", signUp, &signUp.VerificationKey},
                {"change email", changeEmail, &changeEmail.VerificationKey},
            }

            for _, target := range targets {
                t.Run(target.name, func(t *testing.T) {
                    originalKey := *target.key
                    if len(originalKey) != 64 {
                        t.Fatal("constructor did not generate a verification key")
                    }
                    if err := json.Unmarshal([]byte(input.body), target.action); err != nil {
                        t.Fatalf("decode request: %v", err)
                    }
                    if *target.key != originalKey {
                        t.Fatal("request replaced the server-generated verification key")
                    }
                })
            }

            if signIn.Email != "member@example.com" || signUp.Email != "member@example.com" || changeEmail.Email != "member@example.com" {
                t.Fatal("request email was not decoded")
            }
        })
    }
}

func TestAuthenticationIdentityCannotBeSuppliedByJSON(t *testing.T) {
    signUp := actions.NewCreateTenant()
    body := `{"name":"Member","email":"member@example.com","tenantName":"Community","UserClaims":{"oauth/id":"forged","oauth/email":"admin@example.com"}}`
    if err := json.Unmarshal([]byte(body), signUp); err != nil {
        t.Fatal(err)
    }
    if signUp.UserClaims != nil {
        t.Fatal("request supplied OAuth claims")
    }
    if signUp.Name != "Member" || signUp.Email != "member@example.com" || signUp.TenantName != "Community" {
        t.Fatal("signup input was not decoded")
    }

    changeEmail := actions.NewChangeUserEmail()
    requestor := &entity.User{ID: 12}
    changeEmail.Requestor = requestor
    body = `{"email":"new@example.com","Requestor":{"id":99}}`
    if err := json.Unmarshal([]byte(body), changeEmail); err != nil {
        t.Fatal(err)
    }
    if changeEmail.Requestor != requestor || requestor.ID != 12 {
        t.Fatal("request changed the authenticated requester")
    }
    if changeEmail.Email != "new@example.com" {
        t.Fatal("new email address was not decoded")
    }
}

func TestInvitationStateCannotBeSuppliedByJSON(t *testing.T) {
    for _, sample := range []bool{false, true} {
        action := &actions.InviteUsers{IsSampleInvite: sample}
        body := `{"subject":"Join us","message":"Use %invite%","recipients":["member@example.com"],"Invitations":[{"Email":"admin@example.com","VerificationKey":"chosen-token"}]}`
        if err := json.Unmarshal([]byte(body), action); err != nil {
            t.Fatal(err)
        }
        if action.Invitations != nil {
            t.Fatal("request supplied prepared invitations")
        }

        body = `{"IsSampleInvite":true}`
        if sample {
            body = `{"IsSampleInvite":false}`
        }
        if err := json.Unmarshal([]byte(body), action); err != nil {
            t.Fatal(err)
        }
        if action.IsSampleInvite != sample {
            t.Fatal("request changed the handler's invitation mode")
        }
        if action.Subject != "Join us" || action.Message != "Use %invite%" || len(action.Recipients) != 1 || action.Recipients[0] != "member@example.com" {
            t.Fatal("invitation input was not decoded")
        }
    }
}
