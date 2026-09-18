package application

import (
	"bytes"
	"context"
	"testing"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)
type testSecretBox struct{}
func(testSecretBox)Encrypt(input []byte)([]byte,error){return append([]byte("encrypted:"),input...),nil}
func(testSecretBox)Decrypt(input []byte)([]byte,error){return bytes.TrimPrefix(input,[]byte("encrypted:")),nil}
func TestCredentialServiceStoresEncryptedSSHCredential(t *testing.T){
	repo:=NewMemoryCredentialRepository();service:=NewCredentialService(repo,testSecretBox{})
	id,err:=service.StoreSSH(context.Background(),domain.SSHCredential{PrivateKey:"secret-key",Passphrase:"secret-pass"});if err!=nil{t.Fatal(err)}
	record,err:=repo.Get(context.Background(),id);if err!=nil{t.Fatal(err)}
	if !bytes.HasPrefix(record.Ciphertext,[]byte("encrypted:")){t.Fatal("credential was not encrypted")}
	credential,err:=service.GetSSH(context.Background(),id);if err!=nil{t.Fatal(err)}
	if credential.PrivateKey!="secret-key"||credential.Passphrase!="secret-pass"{t.Fatalf("unexpected credential: %#v",credential)}
}
