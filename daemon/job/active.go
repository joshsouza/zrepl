package job

import (
	"context"
	"github.com/pkg/errors"
	"github.com/problame/go-streamrpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zrepl/zrepl/config"
	"github.com/zrepl/zrepl/daemon/connecter"
	"github.com/zrepl/zrepl/daemon/filters"
	"github.com/zrepl/zrepl/daemon/pruner"
	"github.com/zrepl/zrepl/endpoint"
	"github.com/zrepl/zrepl/replication"
	"sync"
	"github.com/zrepl/zrepl/daemon/logging"
	"github.com/zrepl/zrepl/daemon/snapper"
)

type ActiveSide struct {
	mode          activeMode
	name          string
	clientFactory *connecter.ClientFactory

	prunerFactory *pruner.PrunerFactory


	promRepStateSecs *prometheus.HistogramVec // labels: state
	promPruneSecs *prometheus.HistogramVec // labels: prune_side
	promBytesReplicated *prometheus.CounterVec // labels: filesystem

	mtx         sync.Mutex
	replication *replication.Replication
}

type activeMode interface {
	SenderReceiver(client *streamrpc.Client) (replication.Sender, replication.Receiver, error)
	Type() Type
	RunPeriodic(ctx context.Context, wakeUpCommon chan<- struct{})
}

type modePush struct {
	fsfilter         endpoint.FSFilter
	snapper *snapper.Snapper
}

func (m *modePush) SenderReceiver(client *streamrpc.Client) (replication.Sender, replication.Receiver, error) {
	sender := endpoint.NewSender(m.fsfilter)
	receiver := endpoint.NewRemote(client)
	return sender, receiver, nil
}

func (m *modePush) Type() Type { return TypePush }

func (m *modePush) RunPeriodic(ctx context.Context, wakeUpCommon chan <- struct{}) {
	m.snapper.Run(ctx, wakeUpCommon)
}


func modePushFromConfig(g *config.Global, in *config.PushJob) (*modePush, error) {
	m := &modePush{}
	fsf, err := filters.DatasetMapFilterFromConfig(in.Filesystems)
	if err != nil {
		return nil, errors.Wrap(err, "cannnot build filesystem filter")
	}
	m.fsfilter = fsf

	if m.snapper, err = snapper.FromConfig(g, fsf, &in.Snapshotting); err != nil {
		return nil, errors.Wrap(err, "cannot build snapper")
	}

	return m, nil
}

func activeSide(g *config.Global, in *config.ActiveJob, mode activeMode) (j *ActiveSide, err error) {

	j = &ActiveSide{mode: mode}
	j.name = in.Name
	j.promRepStateSecs = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "zrepl",
		Subsystem:   "replication",
		Name:        "state_time",
		Help:        "seconds spent during replication",
		ConstLabels: prometheus.Labels{"zrepl_job":j.name},
	}, []string{"state"})
	j.promBytesReplicated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   "zrepl",
		Subsystem:   "replication",
		Name:        "bytes_replicated",
		Help:        "number of bytes replicated from sender to receiver per filesystem",
		ConstLabels: prometheus.Labels{"zrepl_job":j.name},
	}, []string{"filesystem"})

	j.clientFactory, err = connecter.FromConfig(g, in.Connect)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build client")
	}

	j.promPruneSecs = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "zrepl",
		Subsystem:   "pruning",
		Name:        "time",
		Help:        "seconds spent in pruner",
		ConstLabels: prometheus.Labels{"zrepl_job":j.name},
	}, []string{"prune_side"})
	j.prunerFactory, err = pruner.NewPrunerFactory(in.Pruning, j.promPruneSecs)
	if err != nil {
		return nil, err
	}

	return j, nil
}

func (j *ActiveSide) RegisterMetrics(registerer prometheus.Registerer) {
	registerer.MustRegister(j.promRepStateSecs)
	registerer.MustRegister(j.promPruneSecs)
	registerer.MustRegister(j.promBytesReplicated)
}

func (j *ActiveSide) Name() string { return j.name }

type ActiveSideStatus struct {
	Replication *replication.Report
}

func (j *ActiveSide) Status() *Status {
	rep := func() *replication.Replication {
		j.mtx.Lock()
		defer j.mtx.Unlock()
		if j.replication == nil {
			return nil
		}
		return j.replication
	}()
	s := &ActiveSideStatus{}
	t := j.mode.Type()
	if rep == nil {
		return &Status{Type: t, JobSpecific: s}
	}
	s.Replication = rep.Report()
	return &Status{Type: t, JobSpecific: s}
}

func (j *ActiveSide) Run(ctx context.Context) {
	log := GetLogger(ctx)
	ctx = logging.WithSubsystemLoggers(ctx, log)

	defer log.Info("job exiting")

	periodicDone := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go j.mode.RunPeriodic(ctx, periodicDone)

	invocationCount := 0
outer:
	for {
		log.Info("wait for wakeups")
		select {
		case <-ctx.Done():
			log.WithError(ctx.Err()).Info("context")
			break outer

		case <-WaitWakeup(ctx):
		case <-periodicDone:
		}
		invocationCount++
		invLog := log.WithField("invocation", invocationCount)
		j.do(WithLogger(ctx, invLog))
	}
}

func (j *ActiveSide) do(ctx context.Context) {

	log := GetLogger(ctx)
	ctx = logging.WithSubsystemLoggers(ctx, log)

	client, err := j.clientFactory.NewClient()
	if err != nil {
		log.WithError(err).Error("factory cannot instantiate streamrpc client")
	}
	defer client.Close(ctx)

	sender, receiver, err := j.mode.SenderReceiver(client)

	j.mtx.Lock()
	j.replication = replication.NewReplication(j.promRepStateSecs, j.promBytesReplicated)
	j.mtx.Unlock()

	log.Info("start replication")
	j.replication.Drive(ctx, sender, receiver)

	log.Info("start pruning sender")
	senderPruner := j.prunerFactory.BuildSenderPruner(ctx, sender, sender)
	senderPruner.Prune()

	log.Info("start pruning receiver")
	receiverPruner := j.prunerFactory.BuildReceiverPruner(ctx, receiver, sender)
	receiverPruner.Prune()

}