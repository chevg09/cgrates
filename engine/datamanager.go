/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package engine

import (
	"fmt"
	"strings"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/utils"
	"github.com/cgrates/ltcache"
	"github.com/cgrates/rpcclient"
)

var (
	filterIndexesPrefixMap = utils.StringMap{
		utils.AttributeFilterIndexes:  true,
		utils.ResourceFilterIndexes:   true,
		utils.StatFilterIndexes:       true,
		utils.ThresholdFilterIndexes:  true,
		utils.RouteFilterIndexes:      true,
		utils.ChargerFilterIndexes:    true,
		utils.DispatcherFilterIndexes: true,
	}
	loadCachePrefixMap = utils.StringMap{
		utils.DESTINATION_PREFIX:         true,
		utils.REVERSE_DESTINATION_PREFIX: true,
		utils.RATING_PLAN_PREFIX:         true,
		utils.RATING_PROFILE_PREFIX:      true,
		utils.ACTION_PREFIX:              true,
		utils.ACTION_PLAN_PREFIX:         true,
		utils.ACTION_TRIGGER_PREFIX:      true,
		utils.SHARED_GROUP_PREFIX:        true,
		utils.StatQueuePrefix:            true,
		utils.StatQueueProfilePrefix:     true,
		utils.ThresholdPrefix:            true,
		utils.ThresholdProfilePrefix:     true,
		utils.FilterPrefix:               true,
		utils.RouteProfilePrefix:         true,
		utils.AttributeProfilePrefix:     true,
		utils.ChargerProfilePrefix:       true,
		utils.DispatcherProfilePrefix:    true,
		utils.DispatcherHostPrefix:       true,
	}
	cachePrefixMap = utils.StringMap{
		utils.DESTINATION_PREFIX:         true,
		utils.REVERSE_DESTINATION_PREFIX: true,
		utils.RATING_PLAN_PREFIX:         true,
		utils.RATING_PROFILE_PREFIX:      true,
		utils.ACTION_PREFIX:              true,
		utils.ACTION_PLAN_PREFIX:         true,
		utils.AccountActionPlansPrefix:   true,
		utils.ACTION_TRIGGER_PREFIX:      true,
		utils.SHARED_GROUP_PREFIX:        true,
		utils.ResourceProfilesPrefix:     true,
		utils.TimingsPrefix:              true,
		utils.ResourcesPrefix:            true,
		utils.StatQueuePrefix:            true,
		utils.StatQueueProfilePrefix:     true,
		utils.ThresholdPrefix:            true,
		utils.ThresholdProfilePrefix:     true,
		utils.FilterPrefix:               true,
		utils.RouteProfilePrefix:         true,
		utils.AttributeProfilePrefix:     true,
		utils.ChargerProfilePrefix:       true,
		utils.DispatcherProfilePrefix:    true,
		utils.DispatcherHostPrefix:       true,
		utils.AttributeFilterIndexes:     true,
		utils.ResourceFilterIndexes:      true,
		utils.StatFilterIndexes:          true,
		utils.ThresholdFilterIndexes:     true,
		utils.RouteFilterIndexes:         true,
		utils.ChargerFilterIndexes:       true,
		utils.DispatcherFilterIndexes:    true,
	}
)

// NewDataManager returns a new DataManager
func NewDataManager(dataDB DataDB, cacheCfg *config.CacheCfg, connMgr *ConnManager) *DataManager {
	ms, _ := NewMarshaler(config.CgrConfig().GeneralCfg().DBDataEncoding)
	return &DataManager{
		dataDB:   dataDB,
		cacheCfg: cacheCfg,
		connMgr:  connMgr,
		ms:       ms,
	}
}

// DataManager is the data storage manager for CGRateS
// transparently manages data retrieval, further serialization and caching
type DataManager struct {
	dataDB   DataDB
	cacheCfg *config.CacheCfg
	connMgr  *ConnManager
	ms       Marshaler
}

// DataDB exports access to dataDB
func (dm *DataManager) DataDB() DataDB {
	if dm != nil {
		return dm.dataDB
	}
	return nil
}

func (dm *DataManager) LoadDataDBCache(dstIDs, rvDstIDs, rplIDs, rpfIDs, actIDs, aplIDs,
	aaPlIDs, atrgIDs, sgIDs, rpIDs, resIDs, stqIDs, stqpIDs, thIDs, thpIDs, fltrIDs,
	rPrflIDs, alsPrfIDs, cppIDs, dppIDs, dphIDs []string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if dm.DataDB().GetStorageType() == utils.INTERNAL {
		if dm.cacheCfg == nil {
			return
		}
		for k, cacheCfg := range dm.cacheCfg.Partitions {
			k = utils.CacheInstanceToPrefix[k] // alias into prefixes understood by storage
			if loadCachePrefixMap.HasKey(k) && cacheCfg.Precache {
				if err := dm.PreloadCacheForPrefix(k); err != nil && err != utils.ErrInvalidKey {
					return err
				}
			}
		}
		return
	} else {
		for key, ids := range map[string][]string{
			utils.DESTINATION_PREFIX:         dstIDs,
			utils.REVERSE_DESTINATION_PREFIX: rvDstIDs,
			utils.RATING_PLAN_PREFIX:         rplIDs,
			utils.RATING_PROFILE_PREFIX:      rpfIDs,
			utils.ACTION_PREFIX:              actIDs,
			utils.ACTION_PLAN_PREFIX:         aplIDs,
			utils.AccountActionPlansPrefix:   aaPlIDs,
			utils.ACTION_TRIGGER_PREFIX:      atrgIDs,
			utils.SHARED_GROUP_PREFIX:        sgIDs,
			utils.ResourceProfilesPrefix:     rpIDs,
			utils.ResourcesPrefix:            resIDs,
			utils.StatQueuePrefix:            stqIDs,
			utils.StatQueueProfilePrefix:     stqpIDs,
			utils.ThresholdPrefix:            thIDs,
			utils.ThresholdProfilePrefix:     thpIDs,
			utils.FilterPrefix:               fltrIDs,
			utils.RouteProfilePrefix:         rPrflIDs,
			utils.AttributeProfilePrefix:     alsPrfIDs,
			utils.ChargerProfilePrefix:       cppIDs,
			utils.DispatcherProfilePrefix:    dppIDs,
			utils.DispatcherHostPrefix:       dphIDs,
		} {
			if err = dm.CacheDataFromDB(key, ids, false); err != nil {
				return
			}
		}
	}
	return
}

//Used for InternalDB
func (dm *DataManager) PreloadCacheForPrefix(prefix string) error {
	if dm == nil {
		return utils.ErrNoDatabaseConn
	}
	transID := Cache.BeginTransaction()
	Cache.Clear([]string{utils.CachePrefixToInstance[prefix]})
	keyList, err := dm.DataDB().GetKeysForPrefix(prefix)
	if err != nil {
		Cache.RollbackTransaction(transID)
		return err
	}
	switch prefix {
	case utils.RATING_PLAN_PREFIX:
		for _, key := range keyList {
			_, err := dm.GetRatingPlan(key[len(utils.RATING_PLAN_PREFIX):], true, transID)
			if err != nil {
				Cache.RollbackTransaction(transID)
				return err
			}
		}
	default:
		Cache.RollbackTransaction(transID)
		return utils.ErrInvalidKey
	}
	Cache.CommitTransaction(transID)
	return nil
}

func (dm *DataManager) CacheDataFromDB(prfx string, ids []string, mustBeCached bool) (err error) {
	if dm == nil {
		return utils.ErrNoDatabaseConn
	}
	if !cachePrefixMap.HasKey(prfx) {
		return utils.NewCGRError(utils.DataManager,
			utils.MandatoryIEMissingCaps,
			utils.UnsupportedCachePrefix,
			fmt.Sprintf("prefix <%s> is not a supported cache prefix", prfx))
	}
	if ids == nil {
		keyIDs, err := dm.DataDB().GetKeysForPrefix(prfx)
		if err != nil {
			return utils.NewCGRError(utils.DataManager,
				utils.ServerErrorCaps,
				err.Error(),
				fmt.Sprintf("DataManager error <%s> querying keys for prefix: <%s>", err.Error(), prfx))
		}
		for _, keyID := range keyIDs {
			if mustBeCached { // Only consider loading ids which are already in cache
				if _, hasIt := Cache.Get(utils.CachePrefixToInstance[prfx], keyID[len(prfx):]); !hasIt {
					continue
				}
			}
			ids = append(ids, keyID[len(prfx):])
		}
		var nrItems int
		if cCfg, has := dm.cacheCfg.Partitions[utils.CachePrefixToInstance[prfx]]; has {
			nrItems = cCfg.Limit
		}
		if nrItems > 0 && nrItems < len(ids) { // More ids than cache config allows it, limit here
			ids = ids[:nrItems]
		}
	}
	for _, dataID := range ids {
		if mustBeCached {
			if _, hasIt := Cache.Get(utils.CachePrefixToInstance[prfx], dataID); !hasIt { // only cache if previously there
				continue
			}
		}
		switch prfx {
		case utils.DESTINATION_PREFIX:
			_, err = dm.GetDestination(dataID, true, utils.NonTransactional)
		case utils.REVERSE_DESTINATION_PREFIX:
			_, err = dm.GetReverseDestination(dataID, true, utils.NonTransactional)
		case utils.RATING_PLAN_PREFIX:
			_, err = dm.GetRatingPlan(dataID, true, utils.NonTransactional)
		case utils.RATING_PROFILE_PREFIX:
			_, err = dm.GetRatingProfile(dataID, true, utils.NonTransactional)
		case utils.ACTION_PREFIX:
			_, err = dm.GetActions(dataID, true, utils.NonTransactional)
		case utils.ACTION_PLAN_PREFIX:
			_, err = dm.GetActionPlan(dataID, true, utils.NonTransactional)
		case utils.AccountActionPlansPrefix:
			_, err = dm.GetAccountActionPlans(dataID, true, utils.NonTransactional)
		case utils.ACTION_TRIGGER_PREFIX:
			_, err = dm.GetActionTriggers(dataID, true, utils.NonTransactional)
		case utils.SHARED_GROUP_PREFIX:
			_, err = dm.GetSharedGroup(dataID, true, utils.NonTransactional)
		case utils.ResourceProfilesPrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetResourceProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.ResourcesPrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetResource(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.StatQueueProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetStatQueueProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.StatQueuePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetStatQueue(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.TimingsPrefix:
			_, err = dm.GetTiming(dataID, true, utils.NonTransactional)
		case utils.ThresholdProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetThresholdProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.ThresholdPrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetThreshold(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.FilterPrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetFilter(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.RouteProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetRouteProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.AttributeProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetAttributeProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.ChargerProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetChargerProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.DispatcherProfilePrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetDispatcherProfile(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.DispatcherHostPrefix:
			tntID := utils.NewTenantID(dataID)
			_, err = dm.GetDispatcherHost(tntID.Tenant, tntID.ID, false, true, utils.NonTransactional)
		case utils.AttributeFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheAttributeFilterIndexes, dataID)
		case utils.ResourceFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheResourceFilterIndexes, dataID)
		case utils.StatFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheStatFilterIndexes, dataID)
		case utils.ThresholdFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheThresholdFilterIndexes, dataID)
		case utils.RouteFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheRouteFilterIndexes, dataID)
		case utils.ChargerFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheChargerFilterIndexes, dataID)
		case utils.DispatcherFilterIndexes:
			err = dm.MatchFilterIndexFromKey(utils.CacheDispatcherFilterIndexes, dataID)
		case utils.LoadIDPrefix:
			_, err = dm.GetItemLoadIDs(utils.EmptyString, true)
		}
		if err != nil {
			if err == utils.ErrNotFound {
				Cache.Remove(utils.CachePrefixToInstance[prfx], dataID,
					cacheCommit(utils.NonTransactional), utils.NonTransactional)
				err = nil
			} else {
				return utils.NewCGRError(utils.DataManager,
					utils.ServerErrorCaps,
					err.Error(),
					fmt.Sprintf("error <%s> querying DataManager for category: <%s>, dataID: <%s>", err.Error(), prfx, dataID))
			}
		}
	}
	return
}

func (dm *DataManager) GetDestination(key string, skipCache bool, transactionID string) (dest *Destination, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	dest, err = dm.dataDB.GetDestinationDrv(key, skipCache, transactionID)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDestinations]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetDestination, &utils.StringWithApiKey{
					Arg:       key,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &dest); err == nil {
				err = dm.dataDB.SetDestinationDrv(dest, utils.NonTransactional)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			return nil, err
		}
	}
	return
}

func (dm *DataManager) SetDestination(dest *Destination, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.SetDestinationDrv(dest, transactionID); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDestinations]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetDestination,
			&DestinationWithArgDispatcher{
				Destination: dest,
				TenantArg:   utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveDestination(destID string, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.RemoveDestinationDrv(destID, transactionID); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDestinations]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1RemoveDestination,
			&utils.StringWithApiKey{
				Arg:       destID,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) SetReverseDestination(dest *Destination, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.SetReverseDestinationDrv(dest, transactionID); err != nil {
		return
	}
	if config.CgrConfig().DataDbCfg().Items[utils.MetaReverseDestinations].Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetReverseDestination, dest, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) GetReverseDestination(prefix string,
	skipCache bool, transactionID string) (ids []string, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	ids, err = dm.dataDB.GetReverseDestinationDrv(prefix, skipCache, transactionID)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaReverseDestinations]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetReverseDestination, &utils.StringWithApiKey{
					Arg:       prefix,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &ids); err == nil {
				// need to discuss
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			return nil, err
		}
	}
	return
}

func (dm *DataManager) UpdateReverseDestination(oldDest, newDest *Destination,
	transactionID string) error {
	if dm == nil {
		return utils.ErrNoDatabaseConn
	}
	return dm.dataDB.UpdateReverseDestinationDrv(oldDest, newDest, transactionID)
}

func (dm *DataManager) GetAccount(id string) (acc *Account, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	acc, err = dm.dataDB.GetAccountDrv(id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccounts]; err == utils.ErrNotFound &&
			itm.Remote {
			splt := utils.SplitConcatenatedKey(id)
			tenant := utils.FirstNonEmpty(splt[0], config.CgrConfig().GeneralCfg().DefaultTenant)
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetAccount, &utils.StringWithApiKey{
					Arg:       id,
					TenantArg: utils.TenantArg{Tenant: tenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &acc); err == nil {
				err = dm.dataDB.SetAccountDrv(acc)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			return nil, err
		}
	}
	return
}

func (dm *DataManager) SetAccount(acc *Account) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.SetAccountDrv(acc); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccounts]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetAccount,
			&AccountWithArgDispatcher{
				Account: acc,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveAccount(id string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.RemoveAccountDrv(id); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccounts]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveAccount,
			&utils.StringWithApiKey{
				Arg:       id,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

// GetStatQueue retrieves a StatQueue from dataDB
// handles caching and deserialization of metrics
func (dm *DataManager) GetStatQueue(tenant, id string,
	cacheRead, cacheWrite bool, transactionID string) (sq *StatQueue, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheStatQueues, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*StatQueue), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	sq, err = dm.dataDB.GetStatQueueDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueues]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil, utils.ReplicatorSv1GetStatQueue,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, sq); err == nil {
				var ssq *StoredStatQueue
				if dm.dataDB.GetStorageType() != utils.MetaInternal {
					// in case of internal we don't marshal
					if ssq, err = NewStoredStatQueue(sq, dm.ms); err != nil {
						return
					}
				}
				err = dm.dataDB.SetStatQueueDrv(ssq, sq)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheStatQueues, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheStatQueues, tntID, sq, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

// SetStatQueue converts to StoredStatQueue and stores the result in dataDB
func (dm *DataManager) SetStatQueue(sq *StatQueue) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	var ssq *StoredStatQueue
	if dm.dataDB.GetStorageType() != utils.MetaInternal ||
		config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueues].Replicate {
		// in case of internal we don't marshal
		if ssq, err = NewStoredStatQueue(sq, dm.ms); err != nil {
			return
		}
	}
	if err = dm.dataDB.SetStatQueueDrv(ssq, sq); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueues]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetStatQueue,
			&StoredStatQueueWithArgDispatcher{
				StoredStatQueue: ssq,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

// RemoveStatQueue removes the StoredStatQueue
func (dm *DataManager) RemoveStatQueue(tenant, id string, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.RemStatQueueDrv(tenant, id); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueues]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveStatQueue,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

// GetFilter returns a filter based on the given ID
func (dm *DataManager) GetFilter(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (fltr *Filter, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheFilters, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*Filter), nil
		}
	}
	if strings.HasPrefix(id, utils.Meta) {
		fltr, err = NewFilterFromInline(tenant, id)
	} else if dm == nil { // in case we want the filter from dataDB but the connection to dataDB a optional (e.g. SessionS)
		err = utils.ErrNoDatabaseConn
		return
	} else {
		fltr, err = dm.DataDB().GetFilterDrv(tenant, id)
	}
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilters]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil, utils.ReplicatorSv1GetFilter,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &fltr); err == nil {
				err = dm.dataDB.SetFilterDrv(fltr)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheFilters, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheFilters, tntID, fltr, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetFilter(fltr *Filter) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetFilterDrv(fltr); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilters]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetFilter,
			&FilterWithArgDispatcher{
				Filter: fltr,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return

}

func (dm *DataManager) RemoveFilter(tenant, id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveFilterDrv(tenant, id); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilters]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveFilter,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetThreshold(tenant, id string,
	cacheRead, cacheWrite bool, transactionID string) (th *Threshold, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheThresholds, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*Threshold), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	th, err = dm.dataDB.GetThresholdDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholds]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetThreshold, &utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &th); err == nil {
				err = dm.dataDB.SetThresholdDrv(th)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheThresholds, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheThresholds, tntID, th, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetThreshold(th *Threshold) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetThresholdDrv(th); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholds]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetThreshold,
			&ThresholdWithArgDispatcher{
				Threshold: th,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveThreshold(tenant, id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveThresholdDrv(tenant, id); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholds]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1RemoveThreshold,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetThresholdProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (th *ThresholdProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheThresholdProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*ThresholdProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	th, err = dm.dataDB.GetThresholdProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholdProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetThresholdProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &th); err == nil {
				err = dm.dataDB.SetThresholdProfileDrv(th)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheThresholdProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheThresholdProfiles, tntID, th, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetThresholdProfile(th *ThresholdProfile, withIndex bool) (err error) {
	oldTh, err := dm.GetThresholdProfile(th.Tenant, th.ID, true, false, utils.NonTransactional)
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetThresholdProfileDrv(th); err != nil {
		return err
	}
	if withIndex {
		if oldTh != nil {
			var needsRemove bool
			for _, fltrID := range oldTh.FilterIDs {
				if !utils.IsSliceMember(th.FilterIDs, fltrID) {
					needsRemove = true
				}
			}
			if needsRemove {
				if err = NewFilterIndexer(dm, utils.ThresholdProfilePrefix,
					th.Tenant).RemoveItemFromIndex(th.Tenant, th.ID, oldTh.FilterIDs); err != nil {
					return
				}
			}
		}
		if err := createAndIndex(utils.ThresholdProfilePrefix, th.Tenant,
			utils.EmptyString, th.ID, th.FilterIDs, dm); err != nil {
			return err
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholdProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetThresholdProfile,
			&ThresholdProfileWithArgDispatcher{
				ThresholdProfile: th,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID)},
			}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveThresholdProfile(tenant, id,
	transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldTh, err := dm.GetThresholdProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemThresholdProfileDrv(tenant, id); err != nil {
		return
	}
	if oldTh == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		if err = NewFilterIndexer(dm, utils.ThresholdProfilePrefix,
			tenant).RemoveItemFromIndex(tenant, id, oldTh.FilterIDs); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaThresholdProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1RemoveThresholdProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetStatQueueProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (sqp *StatQueueProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheStatQueueProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*StatQueueProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	sqp, err = dm.dataDB.GetStatQueueProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueueProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetStatQueueProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &sqp); err == nil {
				err = dm.dataDB.SetStatQueueProfileDrv(sqp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheStatQueueProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheStatQueueProfiles, tntID, sqp, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetStatQueueProfile(sqp *StatQueueProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldSts, err := dm.GetStatQueueProfile(sqp.Tenant, sqp.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetStatQueueProfileDrv(sqp); err != nil {
		return err
	}
	if withIndex {
		if oldSts != nil {
			var needsRemove bool
			for _, fltrID := range oldSts.FilterIDs {
				if !utils.IsSliceMember(sqp.FilterIDs, fltrID) {
					needsRemove = true
				}
			}
			if needsRemove {
				if err = NewFilterIndexer(dm, utils.StatQueueProfilePrefix,
					sqp.Tenant).RemoveItemFromIndex(sqp.Tenant, sqp.ID, oldSts.FilterIDs); err != nil {
					return
				}
			}
		}
		if err = createAndIndex(utils.StatQueueProfilePrefix, sqp.Tenant,
			utils.EmptyString, sqp.ID, sqp.FilterIDs, dm); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueueProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetStatQueueProfile,
			&StatQueueProfileWithArgDispatcher{
				StatQueueProfile: sqp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveStatQueueProfile(tenant, id,
	transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldSts, err := dm.GetStatQueueProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemStatQueueProfileDrv(tenant, id); err != nil {
		return
	}
	if oldSts == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		if err = NewFilterIndexer(dm, utils.StatQueueProfilePrefix,
			tenant).RemoveItemFromIndex(tenant, id, oldSts.FilterIDs); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaStatQueueProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1RemoveStatQueueProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetTiming(id string, skipCache bool,
	transactionID string) (t *utils.TPTiming, err error) {
	if !skipCache {
		if x, ok := Cache.Get(utils.CacheTimings, id); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*utils.TPTiming), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	t, err = dm.dataDB.GetTimingDrv(id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaTimings]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil, utils.ReplicatorSv1GetTiming,
				&utils.StringWithApiKey{
					Arg:       id,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &t); err == nil {
				err = dm.dataDB.SetTimingDrv(t)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheTimings, id, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheTimings, id, t, nil,
		cacheCommit(transactionID), transactionID)
	return
}

func (dm *DataManager) SetTiming(t *utils.TPTiming) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetTimingDrv(t); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.TimingsPrefix, []string{t.ID}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaTimings]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetTiming,
			&utils.TPTimingWithArgDispatcher{
				TPTiming: t,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveTiming(id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveTimingDrv(id); err != nil {
		return
	}
	Cache.Remove(utils.CacheTimings, id,
		cacheCommit(transactionID), transactionID)
	if config.CgrConfig().DataDbCfg().Items[utils.MetaTimings].Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveTiming, id, &reply)
	}
	return
}

func (dm *DataManager) GetResource(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (rs *Resource, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheResources, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*Resource), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	rs, err = dm.dataDB.GetResourceDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResources]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetResource,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &rs); err == nil {
				err = dm.dataDB.SetResourceDrv(rs)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheResources, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheResources, tntID, rs, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetResource(rs *Resource) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetResourceDrv(rs); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResources]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetResource,
			&ResourceWithArgDispatcher{
				Resource: rs,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveResource(tenant, id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveResourceDrv(tenant, id); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResources]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveResource,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetResourceProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (rp *ResourceProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheResourceProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*ResourceProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	rp, err = dm.dataDB.GetResourceProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResourceProfile]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetResourceProfile, &utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &rp); err == nil {
				err = dm.dataDB.SetResourceProfileDrv(rp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheResourceProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheResourceProfiles, tntID, rp, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetResourceProfile(rp *ResourceProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldRes, err := dm.GetResourceProfile(rp.Tenant, rp.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetResourceProfileDrv(rp); err != nil {
		return err
	}
	if withIndex {
		if oldRes != nil {
			var needsRemove bool
			for _, fltrID := range oldRes.FilterIDs {
				if !utils.IsSliceMember(rp.FilterIDs, fltrID) {
					needsRemove = true
				}
			}
			if needsRemove {
				if err = NewFilterIndexer(dm, utils.ResourceProfilesPrefix,
					rp.Tenant).RemoveItemFromIndex(rp.Tenant, rp.ID, oldRes.FilterIDs); err != nil {
					return
				}
			}
		}
		if err = createAndIndex(utils.ResourceProfilesPrefix, rp.Tenant, utils.EmptyString, rp.ID, rp.FilterIDs, dm); err != nil {
			return
		}
		Cache.Clear([]string{utils.CacheEventResources})
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResourceProfile]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetResourceProfile,
			&ResourceProfileWithArgDispatcher{
				ResourceProfile: rp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveResourceProfile(tenant, id, transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldRes, err := dm.GetResourceProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemoveResourceProfileDrv(tenant, id); err != nil {
		return
	}
	if oldRes == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		if err = NewFilterIndexer(dm, utils.ResourceProfilesPrefix,
			tenant).RemoveItemFromIndex(tenant, id, oldRes.FilterIDs); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaResourceProfile]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveResourceProfile, &utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetActionTriggers(id string, skipCache bool,
	transactionID string) (attrs ActionTriggers, err error) {
	if !skipCache {
		if x, ok := Cache.Get(utils.CacheActionTriggers, id); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(ActionTriggers), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	attrs, err = dm.dataDB.GetActionTriggersDrv(id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionTriggers]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil, utils.ReplicatorSv1GetActionTriggers,
				&utils.StringWithApiKey{
					Arg:       id,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, attrs); err == nil {
				err = dm.dataDB.SetActionTriggersDrv(id, attrs)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheActionTriggers, id, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheActionTriggers, id, attrs, nil,
		cacheCommit(transactionID), transactionID)
	return
}

func (dm *DataManager) RemoveActionTriggers(id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveActionTriggersDrv(id); err != nil {
		return
	}
	Cache.Remove(utils.CacheActionTriggers, id,
		cacheCommit(transactionID), transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionTriggers]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveActionTriggers,
			&utils.StringWithApiKey{
				Arg:       id,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

//SetActionTriggersArgWithArgDispatcher is used to send the key and the ActionTriggers to Replicator
type SetActionTriggersArgWithArgDispatcher struct {
	Key   string
	Attrs ActionTriggers
	utils.TenantArg
	*utils.ArgDispatcher
}

func (dm *DataManager) SetActionTriggers(key string, attr ActionTriggers,
	transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetActionTriggersDrv(key, attr); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.ACTION_TRIGGER_PREFIX, []string{key}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionTriggers]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1SetActionTriggers,
			&SetActionTriggersArgWithArgDispatcher{
				Attrs: attr, Key: key,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID)},
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
			}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) GetSharedGroup(key string, skipCache bool,
	transactionID string) (sg *SharedGroup, err error) {
	if !skipCache {
		if x, ok := Cache.Get(utils.CacheSharedGroups, key); ok {
			if x != nil {
				return x.(*SharedGroup), nil
			}
			return nil, utils.ErrNotFound
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	sg, err = dm.DataDB().GetSharedGroupDrv(key)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaSharedGroups]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetSharedGroup, &utils.StringWithApiKey{
					Arg:       key,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &sg); err == nil {
				err = dm.dataDB.SetSharedGroupDrv(sg)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheSharedGroups, key, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheSharedGroups, key, sg, nil,
		cacheCommit(transactionID), transactionID)
	return
}

func (dm *DataManager) SetSharedGroup(sg *SharedGroup,
	transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetSharedGroupDrv(sg); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.SHARED_GROUP_PREFIX,
		[]string{sg.Id}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaSharedGroups]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetSharedGroup,
			&SharedGroupWithArgDispatcher{
				SharedGroup: sg,
				TenantArg:   utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveSharedGroup(id, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveSharedGroupDrv(id); err != nil {
		return
	}
	Cache.Remove(utils.CacheSharedGroups, id,
		cacheCommit(transactionID), transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaSharedGroups]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveSharedGroup,
			&utils.StringWithApiKey{
				Arg:       id,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetActions(key string, skipCache bool, transactionID string) (as Actions, err error) {
	if !skipCache {
		if x, err := Cache.GetCloned(utils.CacheActions, key); err != nil {
			if err != ltcache.ErrNotFound {
				return nil, err
			}
		} else if x == nil {
			return nil, utils.ErrNotFound
		} else {
			return x.(Actions), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	as, err = dm.DataDB().GetActionsDrv(key)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActions]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetActions, &utils.StringWithApiKey{
					Arg:       key,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &as); err == nil {
				err = dm.dataDB.SetActionsDrv(key, as)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheActions, key, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheActions, key, as, nil,
		cacheCommit(transactionID), transactionID)
	return
}

//SetActionsArgsWithArgDispatcher is used to send the key and the Actions to replicator
type SetActionsArgsWithArgDispatcher struct {
	Key string
	Acs Actions
	utils.TenantArg
	*utils.ArgDispatcher
}

func (dm *DataManager) SetActions(key string, as Actions, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetActionsDrv(key, as); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.ACTION_PREFIX, []string{key}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActions]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil, utils.ReplicatorSv1SetActions,
			&SetActionsArgsWithArgDispatcher{
				Key: key, Acs: as,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveActions(key, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveActionsDrv(key); err != nil {
		return
	}
	Cache.Remove(utils.CacheActions, key,
		cacheCommit(transactionID), transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActions]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveActions, &utils.StringWithApiKey{
				Arg:       key,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetActionPlan(key string, skipCache bool, transactionID string) (ats *ActionPlan, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	ats, err = dm.dataDB.GetActionPlanDrv(key, skipCache, transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionPlans]; err == utils.ErrNotFound && itm.Remote {
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
			utils.ReplicatorSv1GetActionPlan, &utils.StringWithApiKey{
				Arg:       key,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				},
			}, &ats); err == nil {
			err = dm.dataDB.SetActionPlanDrv(key, ats, true, utils.NonTransactional)
		}
	}
	if err != nil {
		err = utils.CastRPCErr(err)
		return nil, err
	}
	return
}

// SetActionPlanArgWithArgDispatcher is used in replicatorV1 for dispatcher
type SetActionPlanArgWithArgDispatcher struct {
	Key       string
	Ats       *ActionPlan
	Overwrite bool
	utils.TenantArg
	*utils.ArgDispatcher
}

func (dm *DataManager) SetActionPlan(key string, ats *ActionPlan,
	overwrite bool, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.SetActionPlanDrv(key, ats, overwrite, transactionID); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionPlans]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetActionPlan, &SetActionPlanArgWithArgDispatcher{
				Key:       key,
				Ats:       ats,
				Overwrite: overwrite,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) GetAllActionPlans() (ats map[string]*ActionPlan, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	ats, err = dm.dataDB.GetAllActionPlansDrv()
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionPlans]; ((err == nil && len(ats) == 0) || err == utils.ErrNotFound) && itm.Remote {
		err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
			utils.ReplicatorSv1GetAllActionPlans,
			&utils.StringWithApiKey{
				Arg:       utils.EmptyString,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				},
			}, &ats)
	}
	if err != nil {
		err = utils.CastRPCErr(err)
		return nil, err
	}
	return
}

func (dm *DataManager) RemoveActionPlan(key string, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.RemoveActionPlanDrv(key, transactionID); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaActionPlans]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveActionPlan,
			&utils.StringWithApiKey{
				Arg:       key,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}
func (dm *DataManager) GetAccountActionPlans(acntID string,
	skipCache bool, transactionID string) (apIDs []string, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	apIDs, err = dm.dataDB.GetAccountActionPlansDrv(acntID, skipCache, transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccountActionPlans]; ((err == nil && len(apIDs) == 0) || err == utils.ErrNotFound) && itm.Remote {
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
			utils.ReplicatorSv1GetAccountActionPlans,
			&utils.StringWithApiKey{
				Arg:       acntID,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				},
			}, &apIDs); err == nil {
			err = dm.dataDB.SetAccountActionPlansDrv(acntID, apIDs, true)
		}
	}
	if err != nil {
		err = utils.CastRPCErr(err)
		return nil, err
	}
	return
}

//SetAccountActionPlansArgWithArgDispatcher is used to send the key and the Actions to replicator
type SetAccountActionPlansArgWithArgDispatcher struct {
	AcntID    string
	AplIDs    []string
	Overwrite bool
	utils.TenantArg
	*utils.ArgDispatcher
}

func (dm *DataManager) SetAccountActionPlans(acntID string, aPlIDs []string, overwrite bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.SetAccountActionPlansDrv(acntID, aPlIDs, overwrite); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccountActionPlans]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetAccountActionPlans, &SetAccountActionPlansArgWithArgDispatcher{
				AcntID:    acntID,
				AplIDs:    aPlIDs,
				Overwrite: overwrite,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

// RemAccountActionPlansArgsWithArgDispatcher is used in replicatorV1 for dispatcher
type RemAccountActionPlansArgsWithArgDispatcher struct {
	AcntID string
	ApIDs  []string
	utils.TenantArg
	*utils.ArgDispatcher
}

func (dm *DataManager) RemAccountActionPlans(acntID string, apIDs []string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.dataDB.RemAccountActionPlansDrv(acntID, apIDs); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAccountActionPlans]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemAccountActionPlans,
			&RemAccountActionPlansArgsWithArgDispatcher{
				AcntID: acntID, ApIDs: apIDs,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetRatingPlan(key string, skipCache bool,
	transactionID string) (rp *RatingPlan, err error) {
	if !skipCache {
		if x, ok := Cache.Get(utils.CacheRatingPlans, key); ok {
			if x != nil {
				return x.(*RatingPlan), nil
			}
			return nil, utils.ErrNotFound
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	rp, err = dm.DataDB().GetRatingPlanDrv(key)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingPlans]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetRatingPlan,
				&utils.StringWithApiKey{
					Arg:       key,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &rp); err == nil {
				err = dm.dataDB.SetRatingPlanDrv(rp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheRatingPlans, key, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheRatingPlans, key, rp, nil,
		cacheCommit(transactionID), transactionID)
	return
}

func (dm *DataManager) SetRatingPlan(rp *RatingPlan, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetRatingPlanDrv(rp); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.RATING_PLAN_PREFIX, []string{rp.Id}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingPlans]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetRatingPlan,
			&RatingPlanWithArgDispatcher{
				RatingPlan: rp,
				TenantArg:  utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveRatingPlan(key string, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveRatingPlanDrv(key); err != nil {
		return
	}
	Cache.Remove(utils.CacheRatingPlans, key,
		cacheCommit(transactionID), transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingPlans]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveRatingPlan,
			&utils.StringWithApiKey{
				Arg:       key,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

// GetRatingProfile returns the RatingProfile for the key
func (dm *DataManager) GetRatingProfile(key string, skipCache bool,
	transactionID string) (rpf *RatingProfile, err error) {
	if !skipCache {
		for _, cacheRP := range []string{utils.CacheRatingProfilesTmp, utils.CacheRatingProfiles} {
			if x, ok := Cache.Get(cacheRP, key); ok {
				if x != nil {
					return x.(*RatingProfile), nil
				}
				return nil, utils.ErrNotFound
			}
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	rpf, err = dm.DataDB().GetRatingProfileDrv(key)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetRatingProfile,
				&utils.StringWithApiKey{
					Arg:       key,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &rpf); err == nil {
				err = dm.dataDB.SetRatingProfileDrv(rpf)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(utils.CacheRatingProfiles, key, nil, nil,
					cacheCommit(transactionID), transactionID)
			}
			return nil, err
		}
	}
	Cache.Set(utils.CacheRatingProfiles, key, rpf, nil,
		cacheCommit(transactionID), transactionID)
	return
}

func (dm *DataManager) SetRatingProfile(rpf *RatingProfile,
	transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetRatingProfileDrv(rpf); err != nil {
		return
	}
	if err = dm.CacheDataFromDB(utils.RATING_PROFILE_PREFIX, []string{rpf.Id}, true); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetRatingProfile,
			&RatingProfileWithArgDispatcher{
				RatingProfile: rpf,
				TenantArg:     utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveRatingProfile(key string,
	transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveRatingProfileDrv(key); err != nil {
		return
	}
	Cache.Remove(utils.CacheRatingProfiles, key,
		cacheCommit(transactionID), transactionID)
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRatingProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveRatingProfile,
			&utils.StringWithApiKey{
				Arg:       key,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) HasData(category, subject, tenant string) (has bool, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	return dm.DataDB().HasDataDrv(category, subject, tenant)
}

func (dm *DataManager) GetFilterIndexes(cacheID, itemIDPrefix, filterType string,
	fldNameVal map[string]string) (indexes map[string]utils.StringMap, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if indexes, err = dm.DataDB().GetFilterIndexesDrv(cacheID, itemIDPrefix, filterType, fldNameVal); err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilterIndexes]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetFilterIndexes,
				&utils.GetFilterIndexesArgWithArgDispatcher{
					GetFilterIndexesArg: &utils.GetFilterIndexesArg{
						CacheID:      cacheID,
						ItemIDPrefix: itemIDPrefix,
						FilterType:   filterType,
						FldNameVal:   fldNameVal},
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID)},
				}, &indexes); err == nil {
				err = dm.dataDB.SetFilterIndexesDrv(cacheID, itemIDPrefix, indexes, true, utils.NonTransactional)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			return nil, err
		}
	}
	return
}

func (dm *DataManager) SetFilterIndexes(cacheID, itemIDPrefix string,
	indexes map[string]utils.StringMap, commit bool, transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetFilterIndexesDrv(cacheID, itemIDPrefix,
		indexes, commit, transactionID); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilterIndexes]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetFilterIndexes,
			&utils.SetFilterIndexesArgWithArgDispatcher{
				SetFilterIndexesArg: &utils.SetFilterIndexesArg{
					CacheID:      cacheID,
					ItemIDPrefix: itemIDPrefix,
					Indexes:      indexes},
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID)},
			}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveFilterIndexes(cacheID, itemIDPrefix string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().RemoveFilterIndexesDrv(cacheID, itemIDPrefix); err != nil {
		return
	}
	return
}

func (dm *DataManager) MatchFilterIndexFromKey(cacheID, key string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	splt := utils.SplitConcatenatedKey(key) // prefix:filterType:fieldName:fieldVal
	lsplt := len(splt)
	if lsplt < 4 {
		return utils.ErrNotFound
	}
	fieldVal := splt[lsplt-1]
	fieldName := splt[lsplt-2]
	filterType := splt[lsplt-3]
	itemIDPrefix := utils.ConcatenatedKey(splt[:lsplt-3]...) // prefix may contain context/subsystems
	_, err = dm.MatchFilterIndex(cacheID, itemIDPrefix, filterType, fieldName, fieldVal)
	return
}

func (dm *DataManager) MatchFilterIndex(cacheID, itemIDPrefix,
	filterType, fieldName, fieldVal string) (itemIDs utils.StringMap, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	fieldValKey := utils.ConcatenatedKey(itemIDPrefix, filterType, fieldName, fieldVal)
	if x, ok := Cache.Get(cacheID, fieldValKey); ok { // Attempt to find in cache first
		if x == nil {
			return nil, utils.ErrNotFound
		}
		return x.(utils.StringMap), nil
	}
	// Not found in cache, check in DB
	itemIDs, err = dm.DataDB().MatchFilterIndexDrv(cacheID, itemIDPrefix, filterType, fieldName, fieldVal)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaFilterIndexes]; err == utils.ErrNotFound && itm.Remote {
			err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1MatchFilterIndex,
				&utils.MatchFilterIndexArgWithArgDispatcher{MatchFilterIndexArg: &utils.MatchFilterIndexArg{
					CacheID:      cacheID,
					ItemIDPrefix: itemIDPrefix,
					FilterType:   filterType,
					FieldName:    fieldName,
					FieldVal:     fieldVal,
				},
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &itemIDs)
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound {
				Cache.Set(cacheID, fieldValKey, nil, nil,
					true, utils.NonTransactional)

			}
			return nil, err
		}
	}
	Cache.Set(cacheID, fieldValKey, itemIDs, nil,
		true, utils.NonTransactional)
	return
}

func (dm *DataManager) GetRouteProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (rpp *RouteProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheRouteProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*RouteProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	rpp, err = dm.dataDB.GetRouteProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRouteProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil, utils.ReplicatorSv1GetRouteProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &rpp); err == nil {
				err = dm.dataDB.SetRouteProfileDrv(rpp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheRouteProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	// populate cache will compute specific config parameters
	if err = rpp.Compile(); err != nil {
		return nil, err
	}
	if cacheWrite {
		Cache.Set(utils.CacheRouteProfiles, tntID, rpp, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetRouteProfile(rpp *RouteProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldRpp, err := dm.GetRouteProfile(rpp.Tenant, rpp.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetRouteProfileDrv(rpp); err != nil {
		return err
	}
	if withIndex {
		if oldRpp != nil {
			var needsRemove bool
			for _, fltrID := range oldRpp.FilterIDs {
				if !utils.IsSliceMember(rpp.FilterIDs, fltrID) {
					needsRemove = true
				}
			}
			if needsRemove {
				if err = NewFilterIndexer(dm, utils.RouteProfilePrefix,
					rpp.Tenant).RemoveItemFromIndex(rpp.Tenant, rpp.ID, oldRpp.FilterIDs); err != nil {
					return
				}
			}
		}
		if err = createAndIndex(utils.RouteProfilePrefix, rpp.Tenant,
			utils.EmptyString, rpp.ID, rpp.FilterIDs, dm); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRouteProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetRouteProfile,
			&RouteProfileWithArgDispatcher{
				RouteProfile: rpp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveRouteProfile(tenant, id, transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldRpp, err := dm.GetRouteProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemoveRouteProfileDrv(tenant, id); err != nil {
		return
	}
	if oldRpp == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		if err = NewFilterIndexer(dm, utils.RouteProfilePrefix,
			tenant).RemoveItemFromIndex(tenant, id, oldRpp.FilterIDs); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaRouteProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveRouteProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

// GetAttributeProfile returns the AttributeProfile with the given id
func (dm *DataManager) GetAttributeProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (attrPrfl *AttributeProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheAttributeProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*AttributeProfile), nil
		}
	}
	isInline := false
	for typeAttr := range utils.AttrInlineTypes.Data() {
		if strings.HasPrefix(id, typeAttr) {
			isInline = true
			break
		}
	}
	if isInline {
		attrPrfl, err = NewAttributeFromInline(tenant, id)
	} else if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	} else {
		attrPrfl, err = dm.dataDB.GetAttributeProfileDrv(tenant, id)
	}
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAttributeProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetAttributeProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &attrPrfl); err == nil {
				err = dm.dataDB.SetAttributeProfileDrv(attrPrfl)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheAttributeProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if err = attrPrfl.Compile(); err != nil {
		return nil, err
	}
	if cacheWrite {
		Cache.Set(utils.CacheAttributeProfiles, tntID, attrPrfl, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetAttributeProfile(ap *AttributeProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldAP, err := dm.GetAttributeProfile(ap.Tenant, ap.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetAttributeProfileDrv(ap); err != nil {
		return err
	}
	if withIndex {
		if oldAP != nil {
			for _, ctx := range oldAP.Contexts {
				var needsRemove bool
				if !utils.IsSliceMember(ap.Contexts, ctx) {
					needsRemove = true
				} else {
					for _, fltrID := range oldAP.FilterIDs {
						if !utils.IsSliceMember(ap.FilterIDs, fltrID) {
							needsRemove = true
						}
					}
				}
				if needsRemove {
					if err = NewFilterIndexer(dm, utils.AttributeProfilePrefix,
						utils.ConcatenatedKey(ap.Tenant, ctx)).RemoveItemFromIndex(ap.Tenant, ap.ID, oldAP.FilterIDs); err != nil {
						return
					}
				}
			}
		}
		for _, ctx := range ap.Contexts {
			if err = createAndIndex(utils.AttributeProfilePrefix,
				ap.Tenant, ctx, ap.ID, ap.FilterIDs, dm); err != nil {
				return
			}
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAttributeProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetAttributeProfile,
			&AttributeProfileWithArgDispatcher{
				AttributeProfile: ap,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveAttributeProfile(tenant, id string, transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldAttr, err := dm.GetAttributeProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil {
		return err
	}
	if err = dm.DataDB().RemoveAttributeProfileDrv(tenant, id); err != nil {
		return
	}
	if oldAttr == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		for _, context := range oldAttr.Contexts {
			if err = NewFilterIndexer(dm, utils.AttributeProfilePrefix,
				utils.ConcatenatedKey(tenant, context)).RemoveItemFromIndex(tenant, id, oldAttr.FilterIDs); err != nil {
				return
			}
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaAttributeProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveAttributeProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetChargerProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (cpp *ChargerProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheChargerProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*ChargerProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	cpp, err = dm.dataDB.GetChargerProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaChargerProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetChargerProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &cpp); err == nil {
				err = dm.dataDB.SetChargerProfileDrv(cpp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheChargerProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheChargerProfiles, tntID, cpp, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetChargerProfile(cpp *ChargerProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldCpp, err := dm.GetChargerProfile(cpp.Tenant, cpp.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetChargerProfileDrv(cpp); err != nil {
		return err
	}
	if withIndex {
		if oldCpp != nil {
			var needsRemove bool
			for _, fltrID := range oldCpp.FilterIDs {
				if !utils.IsSliceMember(cpp.FilterIDs, fltrID) {
					needsRemove = true
				}
			}
			if needsRemove {
				if err = NewFilterIndexer(dm, utils.ChargerProfilePrefix,
					cpp.Tenant).RemoveItemFromIndex(cpp.Tenant, cpp.ID, oldCpp.FilterIDs); err != nil {
					return
				}
			}
		}
		if err = createAndIndex(utils.ChargerProfilePrefix, cpp.Tenant,
			utils.EmptyString, cpp.ID, cpp.FilterIDs, dm); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaChargerProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetChargerProfile,
			&ChargerProfileWithArgDispatcher{
				ChargerProfile: cpp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveChargerProfile(tenant, id string,
	transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldCpp, err := dm.GetChargerProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemoveChargerProfileDrv(tenant, id); err != nil {
		return
	}
	if oldCpp == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		if err = NewFilterIndexer(dm, utils.ChargerProfilePrefix,
			tenant).RemoveItemFromIndex(tenant, id, oldCpp.FilterIDs); err != nil {
			return
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaChargerProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveChargerProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetDispatcherProfile(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (dpp *DispatcherProfile, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheDispatcherProfiles, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*DispatcherProfile), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	dpp, err = dm.dataDB.GetDispatcherProfileDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherProfiles]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetDispatcherProfile,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &dpp); err == nil {
				err = dm.dataDB.SetDispatcherProfileDrv(dpp)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheDispatcherProfiles, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		Cache.Set(utils.CacheDispatcherProfiles, tntID, dpp, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetDispatcherProfile(dpp *DispatcherProfile, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldDpp, err := dm.GetDispatcherProfile(dpp.Tenant, dpp.ID, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().SetDispatcherProfileDrv(dpp); err != nil {
		return err
	}
	if withIndex {
		if oldDpp != nil {
			for _, ctx := range oldDpp.Subsystems {
				var needsRemove bool
				if !utils.IsSliceMember(dpp.Subsystems, ctx) {
					needsRemove = true
				} else {
					for _, fltrID := range oldDpp.FilterIDs {
						if !utils.IsSliceMember(dpp.FilterIDs, fltrID) {
							needsRemove = true
						}
					}
				}
				if needsRemove {
					if err = NewFilterIndexer(dm, utils.DispatcherProfilePrefix,
						utils.ConcatenatedKey(dpp.Tenant, ctx)).RemoveItemFromIndex(dpp.Tenant, dpp.ID, oldDpp.FilterIDs); err != nil {
						return
					}
				}
			}
		}
		for _, ctx := range dpp.Subsystems {
			if err = createAndIndex(utils.DispatcherProfilePrefix, dpp.Tenant, ctx, dpp.ID, dpp.FilterIDs, dm); err != nil {
				return
			}
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherProfiles]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetDispatcherProfile,
			&DispatcherProfileWithArgDispatcher{
				DispatcherProfile: dpp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveDispatcherProfile(tenant, id string,
	transactionID string, withIndex bool) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldDpp, err := dm.GetDispatcherProfile(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemoveDispatcherProfileDrv(tenant, id); err != nil {
		return
	}
	if oldDpp == nil {
		return utils.ErrNotFound
	}
	if withIndex {
		for _, ctx := range oldDpp.Subsystems {
			if err = NewFilterIndexer(dm, utils.DispatcherProfilePrefix,
				utils.ConcatenatedKey(tenant, ctx)).RemoveItemFromIndex(tenant, id, oldDpp.FilterIDs); err != nil {
				return
			}
		}
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherProfiles]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveDispatcherProfile,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetDispatcherHost(tenant, id string, cacheRead, cacheWrite bool,
	transactionID string) (dH *DispatcherHost, err error) {
	tntID := utils.ConcatenatedKey(tenant, id)
	if cacheRead {
		if x, ok := Cache.Get(utils.CacheDispatcherHosts, tntID); ok {
			if x == nil {
				return nil, utils.ErrNotFound
			}
			return x.(*DispatcherHost), nil
		}
	}
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	dH, err = dm.dataDB.GetDispatcherHostDrv(tenant, id)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherHosts]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetDispatcherHost,
				&utils.TenantIDWithArgDispatcher{
					TenantID: &utils.TenantID{Tenant: tenant, ID: id},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					}}, &dH); err == nil {
				err = dm.dataDB.SetDispatcherHostDrv(dH)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				Cache.Set(utils.CacheDispatcherHosts, tntID, nil, nil,
					cacheCommit(transactionID), transactionID)

			}
			return nil, err
		}
	}
	if cacheWrite {
		cfg := config.CgrConfig()
		if dH.rpcConn, err = NewRPCPool(
			rpcclient.PoolFirst,
			cfg.TlsCfg().ClientKey,
			cfg.TlsCfg().ClientCerificate, cfg.TlsCfg().CaCertificate,
			cfg.GeneralCfg().ConnectAttempts, cfg.GeneralCfg().Reconnects,
			cfg.GeneralCfg().ConnectTimeout, cfg.GeneralCfg().ReplyTimeout,
			dH.Conns, IntRPC.GetInternalChanel(), false); err != nil {
			return nil, err
		}
		Cache.Set(utils.CacheDispatcherHosts, tntID, dH, nil,
			cacheCommit(transactionID), transactionID)
	}
	return
}

func (dm *DataManager) SetDispatcherHost(dpp *DispatcherHost) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetDispatcherHostDrv(dpp); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherHosts]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetDispatcherHost,
			&DispatcherHostWithArgDispatcher{
				DispatcherHost: dpp,
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

func (dm *DataManager) RemoveDispatcherHost(tenant, id string,
	transactionID string) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	oldDpp, err := dm.GetDispatcherHost(tenant, id, true, false, utils.NonTransactional)
	if err != nil && err != utils.ErrNotFound {
		return err
	}
	if err = dm.DataDB().RemoveDispatcherHostDrv(tenant, id); err != nil {
		return
	}
	if oldDpp == nil {
		return utils.ErrNotFound
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaDispatcherHosts]; itm.Replicate {
		var reply string
		dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1RemoveDispatcherHost,
			&utils.TenantIDWithArgDispatcher{
				TenantID: &utils.TenantID{Tenant: tenant, ID: id},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply)
	}
	return
}

func (dm *DataManager) GetItemLoadIDs(itemIDPrefix string, cacheWrite bool) (loadIDs map[string]int64, err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	loadIDs, err = dm.DataDB().GetItemLoadIDsDrv(itemIDPrefix)
	if err != nil {
		if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaLoadIDs]; err == utils.ErrNotFound && itm.Remote {
			if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RmtConns, nil,
				utils.ReplicatorSv1GetItemLoadIDs,
				&utils.StringWithApiKey{
					Arg:       itemIDPrefix,
					TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
					ArgDispatcher: &utils.ArgDispatcher{
						APIKey:  utils.StringPointer(itm.APIKey),
						RouteID: utils.StringPointer(itm.RouteID),
					},
				}, &loadIDs); err == nil {
				err = dm.dataDB.SetLoadIDsDrv(loadIDs)
			}
		}
		if err != nil {
			err = utils.CastRPCErr(err)
			if err == utils.ErrNotFound && cacheWrite {
				for key, _ := range loadIDs {
					Cache.Set(utils.CacheLoadIDs, key, nil, nil,
						cacheCommit(utils.NonTransactional), utils.NonTransactional)
				}

			}
			return nil, err
		}
	}
	if cacheWrite {
		for key, val := range loadIDs {
			Cache.Set(utils.CacheLoadIDs, key, val, nil,
				cacheCommit(utils.NonTransactional), utils.NonTransactional)
		}
	}
	return
}

func (dm *DataManager) SetLoadIDs(loadIDs map[string]int64) (err error) {
	if dm == nil {
		err = utils.ErrNoDatabaseConn
		return
	}
	if err = dm.DataDB().SetLoadIDsDrv(loadIDs); err != nil {
		return
	}
	if itm := config.CgrConfig().DataDbCfg().Items[utils.MetaLoadIDs]; itm.Replicate {
		var reply string
		if err = dm.connMgr.Call(config.CgrConfig().DataDbCfg().RplConns, nil,
			utils.ReplicatorSv1SetLoadIDs,
			&utils.LoadIDsWithArgDispatcher{
				LoadIDs:   loadIDs,
				TenantArg: utils.TenantArg{Tenant: config.CgrConfig().GeneralCfg().DefaultTenant},
				ArgDispatcher: &utils.ArgDispatcher{
					APIKey:  utils.StringPointer(itm.APIKey),
					RouteID: utils.StringPointer(itm.RouteID),
				}}, &reply); err != nil {
			err = utils.CastRPCErr(err)
			return
		}
	}
	return
}

// Reconnect reconnects to the DB when the config was changed
func (dm *DataManager) Reconnect(marshaller string, newcfg *config.DataDbCfg) (err error) {
	d, err := NewDataDBConn(newcfg.DataDbType, newcfg.DataDbHost, newcfg.DataDbPort, newcfg.DataDbName,
		newcfg.DataDbUser, newcfg.DataDbPass, marshaller, newcfg.DataDbSentinelName, newcfg.Items)
	if err != nil {
		return
	}
	// ToDo: consider locking
	dm.dataDB.Close()
	dm.dataDB = d
	return
}
