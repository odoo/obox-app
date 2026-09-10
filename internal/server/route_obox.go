package server

func registerOboxRoutes(s *Server) {
	s.app.Get("/odoo/", s.obox.HandleLanConnection)
	s.app.Get("/odoo/connect", s.obox.HandleOfflineConnect)
	s.app.Get("/odoo/disconnect", s.obox.HandleDisconnect)
}
