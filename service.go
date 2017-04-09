package envoyds

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/proto"
	"strconv"
	"strings"
)

const (
	REDIS_V1_PREFIX    = "EYV1"
	REDIS_SERVICE_NAME = "SERVICENAME"
	REDIS_REPO_NAME    = "REPONAME"
	REDIS_FIELD        = "META"
	REDIS_DELIMITER    = ":"
	REDIS_BATCH_SIZE   = 10
)

type service struct {
	env   string
	redis *redis.Client
}

func NewEnvoyDS(env string, redisHost string, redisPort int) (*service, error) {
	ds := &service{}
	ds.env = env
	ds.redis = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisHost, redisPort),
		Password: "",
	})
	if err := ds.redis.Ping().Err(); err != nil {
		return nil, err
	}
	return ds, nil
}

func (ds *service) RegisterService(host *Host) error {
	serviceKey := ds.getServiceKey(host.Service, host.IpAddress, int(host.Port))
	repoKey := ds.getRepoKey(host.ServiceRepoName, host.IpAddress, int(host.Port))
	return ds.write(serviceKey, repoKey, host)
}

func (ds *service) GetServicesByName(service string) ([]*Host, error) {
	var (
		hosts  = make([]*Host, 0, REDIS_BATCH_SIZE)
	)
	prefix := ds.getServicePrefix(service)
	if _, err := ds.scanAndHandle(prefix,
		func(serviceKey string) error {
			var host Host
			if err := ds.read(serviceKey, &host);err != nil {
				return err
			}
			hosts = append(hosts, &host)
			return nil
		}); err != nil {
		return hosts, err
	}
	return hosts, nil
}

func (ds *service) GetServicesByRepoName(repoName string) ([]*Host, error) {
	var (
		hosts  = make([]*Host, 0, REDIS_BATCH_SIZE)
	)
	prefix := ds.getRepoPrefix(repoName)
	if _, err := ds.scanAndHandle(prefix,
		func(serviceKey string) error {
			var host Host
			if err := ds.read(serviceKey, &host);err != nil {
				return err
			}
			hosts = append(hosts, &host)
			return nil
		}); err != nil {
		return hosts, err
	}
	return hosts, nil
}

func (ds *service) DeleteService(service, ip string, port int) (int, error) {
	if port == 0 {
		prefix := ds.getServiceIpPrefix(service, ip)
		c := 0
		if c, err := ds.scanAndHandle(prefix, ds.deleteServiceByServiceKey); err != nil {
			return c, err
		}
		if c == 0 {
			return 0, errors.New("cannot find services to delete")
		}
		return c, nil
	} else {
		serviceKey := ds.getServiceKey(service, ip, port)
		if err := ds.deleteServiceByServiceKey(serviceKey); err != nil {
			return 0, err
		}
		return 1, nil
	}
}

func (ds *service) UpdateServiceWeight(service, ip string, port int, weight int32) (int, error) {
	if port == 0 {
		prefix := ds.getServiceIpPrefix(service, ip)
		c := 0
		if c, err := ds.scanAndHandle(prefix,
			func(serviceKey string) error {
				return ds.updateServiceWeightByKey(serviceKey, weight)
			}); err != nil {
			return c, err
		}
		if c == 0 {
			return 0, errors.New("cannot find services to update")
		}
		return c, nil
	} else {
		serviceKey := ds.getServiceKey(service, ip, port)
		if err := ds.updateServiceWeightByKey(serviceKey, weight);err != nil {
			return 0, err
		}
		return 1, nil
	}
}

func (ds *service) updateServiceWeightByKey(serviceKey string, weight int32) error {
	var host Host
	if err := ds.read(serviceKey, &host); err != nil {
		return errors.New("cannot find service to update")
	}
	host.Tags.LoadBalancingWeight = weight
	repoKey := ds.getRepoKey(host.GetServiceRepoName(), host.GetIpAddress(), int(host.GetPort()))
	return ds.write(serviceKey, repoKey, &host)
}

func (ds *service) deleteServiceByServiceKey(serviceKey string) error {
	var host Host
	if err := ds.read(serviceKey, &host); err != nil {
		return err
	}
	c, err := ds.redis.Del(serviceKey).Result()
	if err != nil {
		return err
	}
	if c == 0 {
		return errors.New("cannot find service when deleting")
	} else if c == 1 {
		repoKey := strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_REPO_NAME, host.ServiceRepoName, host.IpAddress, strconv.Itoa(int(host.Port))}, REDIS_DELIMITER)
		if err = ds.redis.Del(repoKey).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (ds *service) scanAndHandle(prefix string, handle func(serviceKey string) error) (int, error) {
	var (
		cursor uint64
		err    error
		serviceKeys []string
		unique = make(map[string]bool, REDIS_BATCH_SIZE)
	)
	for {
		serviceKeys, cursor, err = ds.redis.Scan(cursor, prefix, REDIS_BATCH_SIZE).Result()
		if err != nil {
			return len(unique), err
		}
		for _, serviceKey := range serviceKeys {
			var host Host
			if err := ds.read(serviceKey, &host); err != nil {
				return len(unique), err
			}
			if !unique[serviceKey] {
				errLocal := handle(serviceKey)
				if errLocal != nil {
					err = errLocal
				}
				unique[serviceKey] = true
			}
		}

		if cursor == 0 {
			break
		}
	}
	return len(unique), err
}

func (ds *service) getServiceKey(serviceName, ip string, port int) string {
	return strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_SERVICE_NAME, serviceName, ip, strconv.Itoa(port)}, REDIS_DELIMITER)
}

func  (ds *service) getServicePrefix(serviceName string) string {
	return strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_SERVICE_NAME, serviceName, "*"}, REDIS_DELIMITER)
}

func  (ds *service) getServiceIpPrefix(serviceName, ip string) string {
	return strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_SERVICE_NAME, serviceName, ip, "*"}, REDIS_DELIMITER)
}

func (ds *service) getRepoKey(repoName, ip string, port int) string {
	return strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_REPO_NAME, repoName, ip, strconv.Itoa(port)}, REDIS_DELIMITER)
}

func  (ds *service) getRepoPrefix(repoName string) string {
	return strings.Join([]string{REDIS_V1_PREFIX, ds.env, REDIS_SERVICE_NAME, repoName, "*"}, REDIS_DELIMITER)
}

func (ds *service) write(serviceKey, repoKey string, host *Host) error {
	bs, err := proto.Marshal(host)
	if err != nil {
		return err
	}
	pipe := ds.redis.Pipeline()
	if err = pipe.HSet(serviceKey, REDIS_FIELD, bs).Err(); err != nil {
		return err
	}
	if err = pipe.Expire(serviceKey, HOST_TTL).Err(); err != nil {
		return err
	}
	if err = pipe.HSet(repoKey, REDIS_FIELD, []byte(serviceKey)).Err(); err != nil {
		return err
	}
	if err = pipe.Expire(repoKey, HOST_TTL).Err(); err != nil {
		return err
	}
	if _, err = pipe.Exec(); err != nil {
		return err
	}
	return nil
}

func (ds *service) read(serviceKey string, host *Host) error {
	bs, err := ds.redis.HGet(serviceKey, REDIS_FIELD).Result()
	if err != nil {
		return err
	}
	if err = proto.Unmarshal([]byte(bs), host); err != nil {
		return err
	}
	return nil
}