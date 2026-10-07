package escpos

import "encoding/xml"

type EPOSResponse struct {
	XMLName xml.Name `xml:"response"`
	Success bool     `xml:"success,attr"`
	Code    string   `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

func NewSuccessResponse() EPOSResponse {
	return EPOSResponse{Success: true, Code: "", Status: ""}
}

func NewErrorResponse(code string) EPOSResponse {
	return EPOSResponse{Success: false, Code: code, Status: ""}
}

func (r EPOSResponse) String() string {
	b, _ := xml.Marshal(r)
	return string(b)
}

func (r EPOSResponse) Bytes() []byte {
	b, _ := xml.Marshal(r)
	return b
}
