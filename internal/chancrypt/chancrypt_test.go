package chancrypt

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	key, err := ParseKey("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32 zero bytes base64
	if err != nil {
		t.Fatal(err)
	}
	ct, err := Encrypt(key, []byte(`{"bot_token":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != `{"bot_token":"x"}` {
		t.Fatalf("got %s", pt)
	}
}

func TestParseKeyRejectsShort(t *testing.T) {
	if _, err := ParseKey("short"); err == nil {
		t.Fatal("expected error")
	}
}
