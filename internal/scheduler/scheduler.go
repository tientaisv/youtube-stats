package scheduler

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"youtube-manager/internal/collector"
)

type Scheduler struct {
	cronSpec  string
	collector *collector.Collector
	cron      *cron.Cron
	nextRun   *time.Time
}

func NewScheduler(cronSpec string, collector *collector.Collector) *Scheduler {
	return &Scheduler{
		cronSpec:  cronSpec,
		collector: collector,
		cron:      cron.New(),
	}
}

func (s *Scheduler) Start() error {
	log.Printf("[SCHEDULER] Initializing cron scheduler with spec: %s", s.cronSpec)

	entryID, err := s.cron.AddFunc(s.cronSpec, func() {
		log.Println("[SCHEDULER] Cron triggered! Running automatic data collection...")
		if err := s.collector.CollectAll(); err != nil {
			log.Printf("[SCHEDULER ERROR] Data collection failed: %v", err)
		}
	})

	if err != nil {
		log.Printf("[SCHEDULER WARNING] Invalid cron spec '%s', falling back to every 6 hours ('0 */6 * * *'): %v", s.cronSpec, err)
		entryID, _ = s.cron.AddFunc("0 */6 * * *", func() {
			log.Println("[SCHEDULER] Fallback Cron triggered! Running collection...")
			s.collector.CollectAll()
		})
	}

	s.cron.Start()

	entry := s.cron.Entry(entryID)
	next := entry.Next
	s.nextRun = &next

	log.Printf("[SCHEDULER] Next scheduled collection run: %s", next.Format(time.RFC3339))
	return nil
}

func (s *Scheduler) GetNextRunTime() *time.Time {
	return s.nextRun
}

func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}
