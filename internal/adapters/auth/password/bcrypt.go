package password

import "golang.org/x/crypto/bcrypt"

type Bcrypt struct{ cost int }

func NewBcrypt() Bcrypt { return Bcrypt{cost: bcrypt.DefaultCost} }

func (b Bcrypt) Hash(value string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(value), b.cost)
	return string(hash), err
}

func (Bcrypt) Compare(hash, value string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value)) == nil
}
