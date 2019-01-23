package go_micro_srv_chat

import "errors"

func (req *StreamRequest) Validate() error {
	if len(req.Id) == 0 {
		return errors.New("id is required")
	}
	if len(req.Platform) == 0 {
		return errors.New("platform is required")
	}
	return nil
}
