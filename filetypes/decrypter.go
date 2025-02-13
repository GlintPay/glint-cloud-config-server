package filetypes

type Decrypter interface {
	Decrypt(data []byte) ([]byte, error)
}
