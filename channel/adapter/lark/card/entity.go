package card

type Entity struct {
	Type string `json:"type"` // card
	Data struct {
		CardId string `json:"card_id"`
	} `json:"data"`
}

func NewEntity(cardId string) *Entity {
	return &Entity{
		Type: "card",
		Data: struct {
			CardId string `json:"card_id"`
		}{
			CardId: cardId,
		},
	}
}
