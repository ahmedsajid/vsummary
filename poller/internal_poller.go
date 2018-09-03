package poller

import (
	"time"

	"github.com/gbolo/vsummary/common"
	"github.com/gbolo/vsummary/db"
)

// InternalPoller extends Poller with functionality relevant to
// storing results to a backend directly (not over the vSummary API server).
type InternalPoller struct {
	Poller
	stopSignal chan bool
	backend    db.Backend
}

// NewInternalPoller returns a empty InternalPoller
func NewEmptyInternalPoller() *InternalPoller {
	return &InternalPoller{
		stopSignal: make(chan bool),
	}
}

// NewInternalPoller returns a InternalPoller based from a common.Poller
func NewInternalPoller(c common.Poller) (i *InternalPoller) {
	i = NewEmptyInternalPoller()
	i.Configure(c)
	return
}

// SetBackend allows internalPoller to connect to backend database
func (i *InternalPoller) SetBackend(backend db.Backend) {
	i.backend = backend
}

// StorePollResults will send results directly to backend db and not to vsummary API server
func (i *InternalPoller) StorePollResults(r pollResults) (err []error) {
	appendIfError(&err, i.backend.InsertVcenter(r.Vcenter))
	appendIfError(&err, i.backend.InsertEsxi(r.Esxi))
	appendIfError(&err, i.backend.InsertDatastores(r.Datastore))
	appendIfError(&err, i.backend.InsertVirtualmachines(r.Virtualmachine))
	appendIfError(&err, i.backend.InsertVSwitch(r.VSwitch))
	appendIfError(&err, i.backend.InsertVSwitch(r.Dvs))
	// need to insert portgroups here...
	appendIfError(&err, i.backend.InsertVNics(r.Vnic))
	appendIfError(&err, i.backend.InsertVDisks(r.VDisk))
	appendIfError(&err, i.backend.InsertResourcepools(r.ResourcePool))
	appendIfError(&err, i.backend.InsertDatacenters(r.Datacenter))
	appendIfError(&err, i.backend.InsertFolders(r.Folder))
	appendIfError(&err, i.backend.InsertClusters(r.Cluster))

	return
}

// StopPolling sends the signal to stop the loop in Deamonize
func (i *InternalPoller) StopPolling() {
	i.stopSignal <- true
}

func (i *InternalPoller) PollThenStore() {
	r, errs := i.GetPollResults()
	if len(errs) > 0 {
		log.Warningf(
			"will not store poll results since %d error(s) occurred during polling of: %s",
			len(errs),
			i.Config.URL,
		)
		return
	}
	errs = i.StorePollResults(r)
	if len(errs) > 0 {
		log.Warningf(
			"there were %d errors during storing polling results of: %s",
			len(errs),
			i.Config.URL,
		)
	}
}

// Daemonize is a blocking loop which continues to PollThenStore until
// the channel is closed or poller is marked as disabled.
func (i *InternalPoller) Daemonize() {
	t := time.Tick(defaultPollingInterval)
	log.Infof("start polling of %s", i.Config.URL)
	// this prevents all pollers to go off at the exact same time
	randomizedWait(1, 120)
	i.PollThenStore()

	// start polling until we shouldn't anymore
	for {
		select {
		case <-t:
			if i.Enabled {
				// this prevents all pollers to go off at the exact same time
				randomizedWait(1, 120)
				log.Debugf("executing poll of %s", i.Config.URL)
				i.PollThenStore()
			} else {
				log.Infof("stopping polling of %s", i.Config.URL)
				return
			}
		case <-i.stopSignal:
			log.Infof("stop signal received: stop polling of %s", i.Config.URL)
			return
		}
	}
}
