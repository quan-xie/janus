package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hellofresh/janus/pkg/cors"
	"github.com/hellofresh/janus/pkg/middleware"
	"github.com/hellofresh/janus/pkg/oauth"
	"github.com/hellofresh/janus/pkg/proxy"
	"github.com/hellofresh/janus/pkg/router"
	"github.com/hellofresh/janus/pkg/store"
	"github.com/ulule/limiter"
)

// Loader is responsible for loading all apis form a datastore and configure them in a register
type Loader struct {
	register proxy.Register
	store    store.Store
	accessor *middleware.DatabaseAccessor
	manager  *oauth.Manager
	debug    bool
}

// NewLoader creates a new instance of the api manager
func NewLoader(register proxy.Register, store store.Store, accessor *middleware.DatabaseAccessor, manager *oauth.Manager, debug bool) *Loader {
	return &Loader{register, store, accessor, manager, debug}
}

// Load loads all api specs from a datasource
func (m *Loader) Load() {
	specs := m.getAPISpecs()
	m.RegisterApis(specs)
}

// RegisterApis load application middleware
func (m *Loader) RegisterApis(apiSpecs []*Spec) {
	log.Debug("Loading API configurations")

	for _, referenceSpec := range apiSpecs {
		m.RegisterApi(referenceSpec)
	}
}

func (m *Loader) RegisterApi(referenceSpec *Spec) {
	var skip bool

	//Validates the proxy
	skip = proxy.Validate(referenceSpec.Proxy)
	if false == referenceSpec.Active {
		log.Debug("API is not active, skiping...")
		skip = false
	}

	if skip {
		var handlers []router.Constructor
		if referenceSpec.RateLimit.Enabled {
			rate, err := limiter.NewRateFromFormatted(referenceSpec.RateLimit.Limit)
			if err != nil {
				panic(err)
			}

			limiterStore, err := m.store.ToLimiterStore(referenceSpec.Name)
			if err != nil {
				panic(err)
			}

			handlers = append(handlers, limiter.NewHTTPMiddleware(limiter.NewLimiter(limiterStore, rate)).Handler)
		} else {
			log.Debug("Rate limit is not enabled")
		}

		if referenceSpec.CorsMeta.Enabled {
			handlers = append(handlers, cors.NewMiddleware(referenceSpec.CorsMeta, m.debug).Handler)
		} else {
			log.Debug("CORS is not enabled")
		}

		if referenceSpec.UseOauth2 {
			handlers = append(handlers, oauth.NewKeyExistsMiddleware(m.manager).Handler)
		} else {
			log.Debug("OAuth2 is not enabled")
		}

		m.register.Add(proxy.NewRoute(referenceSpec.Proxy, handlers...))
		log.Debug("API registered")
	} else {
		log.Error("Listen path is empty, skipping...")
	}
}

//getAPISpecs Load application specs from datasource
func (m *Loader) getAPISpecs() []*Spec {
	log.Debug("Using App Configuration from Mongo DB")
	repo, err := NewMongoAppRepository(m.accessor.Session.DB(""))
	if err != nil {
		log.Panic(err)
	}

	definitions, err := repo.FindAll()
	if err != nil {
		log.Panic(err)
	}

	var specs []*Spec
	for _, definition := range definitions {
		spec := new(Spec)
		spec.Definition = definition
		specs = append(specs, spec)
	}

	return specs
}
