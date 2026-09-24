package printer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"obox-app/internal/logger"

	"github.com/google/gousb"
)

// ConnKind is the transport a Printer talks over.
type ConnKind int

const (
	ConnKindUSB ConnKind = iota
	ConnKindLAN
)

const (
	QueueSize    = 100
	WriteTimeout = 5 * time.Second
	ChunkSize    = 8 * 1024 // 8 KB
)

var (
	ErrNotFound  = errors.New("printer not found")
	ErrQueueFull = errors.New("printer queue is full")
)

type JobResult struct {
	OK  bool
	Err error
}

type JobFunc func(p *Printer) JobResult

type Job struct {
	run   JobFunc
	reply chan JobResult
}

// Printer owns a single device connection and serialises writes to it
// through the jobs channel drained by loop.
type Printer struct {
	connectionType ConnKind
	id             *ID
	lanIP          string
	mu             sync.Mutex
	// USB fields
	usbCtx      *gousb.Context
	device      *gousb.Device
	config      *gousb.Config
	iFace       *gousb.Interface
	outEndpoint *gousb.OutEndpoint
	// LAN fields
	tcpConn net.Conn
	jobs    chan Job
}

func newPrinter(id string) *Printer {
	// Check if this is a LAN printer
	if lanIP, ok := DecodeLANPrinterID(id); ok {
		p := &Printer{
			connectionType: ConnKindLAN,
			lanIP:          lanIP,
			jobs:           make(chan Job, QueueSize),
		}
		go p.loop()
		return p
	}

	// USB printer
	var printerID *ID = nil
	if id != "" {
		printerID, _ = decodePrinterID(id)
	}

	p := &Printer{
		connectionType: ConnKindUSB,
		id:             printerID,
		jobs:           make(chan Job, QueueSize),
	}

	logger.Debugf("Created new USB printer instance for ID: %s", p.idToString())
	go p.loop()
	return p
}

func (p *Printer) Enqueue(fn JobFunc, reply chan JobResult) error {
	j := Job{run: fn, reply: reply}
	select {
	case p.jobs <- j:
		logger.Debugf("Enqueued print job for printer %s", p.idToString())
		return nil
	default:
		logger.Warnf("Printer queue full for printer %s", p.idToString())
		return ErrQueueFull
	}
}

func (p *Printer) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureOpen(); err != nil {
		return err
	}

	logger.Debugf("Writing %d bytes to printer %s", len(data), p.idToString())

	if p.connectionType == ConnKindLAN {
		if err := p.tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to set write deadline for LAN printer %s: %w", p.idToString(), err)
		}
		if _, err := p.tcpConn.Write(data); err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to write to LAN printer %s: %w", p.idToString(), err)
		}
		logger.Debugf("Successfully wrote to LAN printer %s", p.idToString())
		return nil
	}

	// USB write
	for len(data) > 0 {
		size := min(len(data), ChunkSize)
		logger.Debugf("USB printer %s writing %d bytes", p.idToString(), size)

		ctx, cancel := context.WithTimeout(context.Background(), WriteTimeout)
		_, err := p.outEndpoint.WriteContext(ctx, data[:size])
		cancel()

		if err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to write %d bytes to USB printer %s: %w", size, p.idToString(), err)
		}

		data = data[size:]
	}
	return nil
}

func (p *Printer) loop() {
	logger.Debugf("Printer loop started for %s with %d jobs", p.idToString(), len(p.jobs))
	for j := range p.jobs {
		result := j.run(p)
		if j.reply != nil {
			j.reply <- result
			close(j.reply)
		}
		if len(p.jobs) == 0 {
			p.close()
		}
	}
}
func (p *Printer) ensureOpen() error {
	if p.connectionType == ConnKindLAN {
		return p.ensureOpenLANLocked()
	}
	return p.ensureOpenUSBLocked()
}

func (p *Printer) ensureOpenLANLocked() error {
	if p.tcpConn != nil {
		logger.Debugf("LAN printer %s already connected", p.idToString())
		return nil // already connected
	}

	addr := net.JoinHostPort(p.lanIP, fmt.Sprintf("%d", LANPort))
	logger.Debugf("Attempting to connect to LAN printer %s at %s", p.idToString(), addr)
	conn, err := net.DialTimeout("tcp", addr, LANConnectTimeout)
	if err != nil {
		logger.Errorf("Failed to connect to LAN printer %s at %s: %v", p.idToString(), addr, err)
		return fmt.Errorf("failed to connect to LAN printer at %s: %w", addr, err)
	}

	p.tcpConn = conn
	return nil
}

func (p *Printer) ensureOpenUSBLocked() error {
	if p.device != nil {
		logger.Debugf("USB printer %s already connected", p.idToString())
		return nil // already connected
	}

	ctx := gousb.NewContext()

	var (
		eps     []EndpointInfo
		findAny = p.id == nil
	)

	devices, err := openDevices(ctx, func(desc *gousb.DeviceDesc) bool {
		if findAny && len(eps) > 0 {
			return false
		}

		if p.id != nil && p.id.VidPid != "" && p.id.VidPid != fmt.Sprintf("%04X:%04X", uint16(desc.Vendor), uint16(desc.Product)) {
			return false
		}

		ep, ok := findPrinterEndpoint(desc)
		if ok {
			eps = append(eps, ep)
			return true
		}
		return false
	})
	if err != nil {
		logger.Warnf("Some matching devices failed to open: %v", err)
	}
	if len(devices) == 0 {
		_ = ctx.Close()
		if err != nil {
			return err
		}
		return ErrNotFound
	}

	var (
		target   *gousb.Device
		targetEP *EndpointInfo
	)
	for i, d := range devices {
		serial, _ := d.SerialNumber()

		match := false
		if findAny {
			match = true
		} else if p.id.Serial != "" {
			match = serial == p.id.Serial
		} else if p.id.Path != "" && p.id.VidPid != "" {
			match = pathToString(d.Desc) == p.id.Path && fmt.Sprintf("%04X:%04X", uint16(d.Desc.Vendor), uint16(d.Desc.Product)) == p.id.VidPid
		}

		if match && target == nil {
			target = d
			ep := eps[i]
			targetEP = &ep
		} else {
			_ = d.Close()
		}
	}
	if target == nil || targetEP == nil {
		_ = ctx.Close()
		return ErrNotFound
	}

	_ = target.SetAutoDetach(true)

	cfg, err := target.Config(targetEP.config)
	if err != nil {
		// Retry without auto-detach.
		_ = target.SetAutoDetach(false)
		cfg, err = target.Config(targetEP.config)
	}
	logger.Debugf("Configuring USB device %s", p.idToString())
	if err != nil {
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	iFace, err := cfg.Interface(targetEP.iFace, targetEP.alternateSetting)
	if err != nil {
		logger.Errorf("Failed to claim USB interface for printer %s: Error: %v", p.idToString(), err)
		_ = cfg.Close()
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	ep, err := iFace.OutEndpoint(targetEP.outEndpoint)
	if err != nil {
		logger.Errorf("Failed to get USB out endpoint for printer %s: Error: %v", p.idToString(), err)
		iFace.Close()
		_ = cfg.Close()
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	p.usbCtx = ctx
	p.device = target
	p.config = cfg
	p.iFace = iFace
	p.outEndpoint = ep
	return nil
}

func (p *Printer) close() {
	p.mu.Lock()
	logger.Debugf("Closing printer %s", p.idToString())
	defer p.mu.Unlock()
	p.closeDeviceLocked()
}

func (p *Printer) closeDeviceLocked() {
	if p.connectionType == ConnKindLAN {
		if p.tcpConn != nil {
			_ = p.tcpConn.Close()
			p.tcpConn = nil
			logger.Debugf("LAN printer %s connection closed", p.idToString())
		}
		return
	}

	// USB close
	if p.device == nil {
		return
	}
	p.iFace.Close()
	_ = p.config.Close()
	_ = p.device.Close()
	_ = p.usbCtx.Close()
	p.device = nil
	p.config = nil
	p.iFace = nil
	p.outEndpoint = nil
	p.usbCtx = nil
	logger.Debugf("USB printer %s device closed", p.idToString())
}

func (p *Printer) idToString() string {
	if p.connectionType == ConnKindLAN {
		return fmt.Sprintf("LAN:%s", p.lanIP)
	}
	if p.id != nil {
		return fmt.Sprintf("USB:%s, %v", p.id.Serial, p.id)
	}
	return "USB:unknown"
}
