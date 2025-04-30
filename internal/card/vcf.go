package card

import (
	"bytes"
	"fmt"
	"strings"

	vc "github.com/emersion/go-vcard"
)

func genVCF(card *Card) ([]byte, error) {
	c := make(vc.Card, 0)
	c.Set(vc.FieldVersion, &vc.Field{
		Value: "3.0",
	})

	var displayName string
	fmt.Println("card.DisplayName: ", card.DisplayName)
	splitDisplayNames := strings.Split(strings.TrimSpace(card.DisplayName), " ")

	fmt.Println("splitDisplayNames: ", splitDisplayNames)
	fmt.Println("len: ", len(splitDisplayNames))
	switch ln := len(splitDisplayNames); ln {
	case 2:
		displayName = fmt.Sprintf("%s;%s;;;", splitDisplayNames[1], splitDisplayNames[0])

	case 3:
		displayName = fmt.Sprintf("%s;%s;;%s;", splitDisplayNames[2], splitDisplayNames[1], splitDisplayNames[0])

	default:
		displayName = card.DisplayName
	}

	c.Set(vc.FieldFormattedName, &vc.Field{
		Value: card.DisplayName,
	})

	c.Set(vc.FieldName, &vc.Field{
		Value: displayName,
	})

	tels := make([]*vc.Field, 0)
	if card.PhoneNumber != "" {
		tels = append(tels, &vc.Field{
			Value: card.PhoneNumber,
			Params: vc.Params{
				vc.ParamType: []string{vc.TypeWork},
			},
		})
	}

	if card.MobileNumber != "" {
		tels = append(tels, &vc.Field{
			Value: card.MobileNumber,
			Params: vc.Params{
				vc.ParamType: []string{vc.TypeCell},
			},
		})
	}
	c[vc.FieldTelephone] = tels

	c.Set(vc.FieldEmail, &vc.Field{
		Value: card.Email,
	})

	c.Set(vc.FieldOrganization, &vc.Field{
		Value: fmt.Sprintf("%s;%s;", card.CompanyName, card.DepartmentName),
	})

	c.Set(vc.FieldTitle, &vc.Field{
		Value: card.PositionName,
	})

	buf := new(bytes.Buffer)
	encoder := vc.NewEncoder(buf)
	if err := encoder.Encode(c); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
