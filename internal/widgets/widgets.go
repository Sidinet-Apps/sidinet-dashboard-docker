package widgets

type Definition struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Provider    string   `json:"provider,omitempty"`
	Sizes       []string `json:"sizes"`
	DefaultSize string   `json:"default_size"`
}
type Registry struct{ items map[string]Definition }

func New() *Registry {
	r := &Registry{items: map[string]Definition{}}
	for _, d := range []Definition{{"system.cpu", "CPU", "Sistema", "system.cpu", []string{"1x1", "2x1", "2x2"}, "2x1"}, {"system.memory", "RAM", "Sistema", "system.memory", []string{"1x1", "2x1", "2x2"}, "2x1"}, {"system.temperature", "Temperatura", "Sistema", "system.temperature", []string{"1x1", "2x1"}, "1x1"}, {"system.uptime", "Uptime", "Sistema", "system.uptime", []string{"1x1", "2x1"}, "1x1"}, {"information.clock", "Reloj", "Información", "", []string{"1x1", "2x1"}, "2x1"}, {"network.interfaces", "Red", "Red", "network.interfaces", []string{"2x1", "2x2", "4x2"}, "2x2"}, {"network.ips", "Direcciones IP", "Red", "network.ips", []string{"2x1", "2x2"}, "2x1"}, {"network.internet", "Internet", "Red", "network.internet", []string{"1x1", "2x1"}, "2x1"}, {"network.dns", "DNS", "Red", "network.dns", []string{"2x1", "2x2"}, "2x1"}, {"storage.disks", "Almacenamiento", "Sistema", "storage.disks", []string{"2x1", "2x2", "4x2"}, "2x2"}, {"application.shortcut", "Aplicación", "Aplicaciones", "", []string{"1x1", "2x1", "2x2"}, "1x1"}, {"layout.group", "Grupo", "Organización", "", []string{"2x2", "4x2", "6x3"}, "4x2"}, {"information.iframe", "Iframe", "Información", "", []string{"4x3", "6x4", "12x6"}, "6x4"}, {"information.json", "JSON/API", "Información", "", []string{"2x2", "4x2", "6x3"}, "4x2"}} {
		r.items[d.Type] = d
	}
	return r
}
func (r *Registry) All() []Definition {
	o := make([]Definition, 0, len(r.items))
	for _, v := range r.items {
		o = append(o, v)
	}
	return o
}
func (r *Registry) Get(t string) (Definition, bool) { v, ok := r.items[t]; return v, ok }
