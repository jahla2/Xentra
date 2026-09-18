package clients
import("bytes";"testing")
func TestAESSecretBoxRoundTripDoesNotStorePlaintext(t *testing.T){box,err:=NewAESSecretBox(bytes.Repeat([]byte{7},32));if err!=nil{t.Fatal(err)};plain:=[]byte("private-key-material");ciphertext,err:=box.Encrypt(plain);if err!=nil{t.Fatal(err)};if bytes.Contains(ciphertext,plain){t.Fatal("ciphertext contains plaintext")};decrypted,err:=box.Decrypt(ciphertext);if err!=nil{t.Fatal(err)};if !bytes.Equal(decrypted,plain){t.Fatalf("unexpected: %q",decrypted)}}
