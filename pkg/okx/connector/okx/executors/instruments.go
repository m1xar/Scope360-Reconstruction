package executors

import (
	"fmt"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const instrumentsPath = "/api/v5/public/instruments"

func FetchInstrument(client *resty.Client, baseURL, instID, instType string) (*models.Instrument, error) {
	params := map[string]string{
		"instType": instType,
		"instId":   instID,
	}

	data, err := okx.DoGet[[]models.Instrument](client, baseURL, instrumentsPath, params)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return &models.DefaultInstrument, nil
	}

	return &data[0], nil
}

func FetchInstrumentsByType(client *resty.Client, baseURL, instType string) ([]models.Instrument, error) {
	return okx.DoGet[[]models.Instrument](client, baseURL, instrumentsPath, map[string]string{"instType": instType})
}

func FetchInstruments(client *resty.Client, baseURL string, identifiers map[string]models.Instrumentidentifier) (map[string]models.Instrument, error) {
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		firstErr    error
		semaphore   = make(chan struct{}, 5)
		instruments = make(map[string]models.Instrument, len(identifiers))
	)

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	for i := range identifiers {
		wg.Add(1)
		id := identifiers[i]
		semaphore <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()

			instrument, fetchErr := FetchInstrument(client, baseURL, id.InstID, id.InstType)
			if fetchErr != nil {
				setErr(fetchErr)
				return
			}
			if instrument == nil {
				setErr(fmt.Errorf("okx: instrument %s returned nil", id.InstID))
				return
			}

			mu.Lock()
			instruments[id.InstID] = *instrument
			mu.Unlock()
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	return instruments, firstErr
}
