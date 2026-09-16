package printer

import (
	"errors"

	"epos-proxy/internal/escpos"
	"epos-proxy/internal/logger"
)

func (m *Manager) PrintReceipt(printerID string, body []byte) escpos.EPOSResponse {
	if m == nil {
		logger.Errorf("Printer manager is not initialized")
		return escpos.NewErrorResponse("EX_BADPORT")
	}

	logger.Debugf("Processing print job for printer: %s", printerID)
	jobData, err := escpos.ParseXML(body)
	if err != nil {
		logger.Errorf("XML parsing error: %v", err)
		return escpos.NewErrorResponse("SchemaError")
	}
	logger.Debug("XML parsed successfully")

	reply, err := m.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}

	if err != nil {
		retCode := ""
		if errors.Is(err, ErrQueueFull) {
			retCode = "TooManyRequests"
			logger.Warn("Printer queue full")
		} else {
			retCode = "EX_BADPORT"
		}
		logger.Errorf("Print error [%s]: %v, Printer ID: %s", retCode, err, printerID)
		return escpos.NewErrorResponse(retCode)
	}

	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return escpos.NewSuccessResponse()
}
