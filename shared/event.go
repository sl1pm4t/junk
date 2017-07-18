package shared

import "math/rand"

type Event map[string]interface{}

func EventType() string {

	r := rand.Intn(6)

	switch r {
	case 0:
		return "Login"
	case 1:
		return "Logout"
	case 2:
		return "Request"
	case 3:
		return "Response"
	case 4:
		return "KeepAlive"
	}

	return "Error"
}
