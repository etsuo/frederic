package frederic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

var ethnicities = map[string]bool{
	"UNK": true,
	"W":   true,
	"B":   true,
	"A":   true,
	"PI":  true,
	"H":   true,
	"O":   true,
}

type useredit struct {
	Ids        []int64
	Aus        []appuser
	DeletedIds []int64
}

func apiuserOK(c appengine.Context, w http.ResponseWriter) bool {
	if !userauthenticated(c) {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	if ok, _ := userauthorized(c, user.Current(c).Email); !ok {
		w.WriteHeader(http.StatusForbidden)
		return false
	}
	return true
}

func addclient(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	clt := &client{}
	body := make([]byte, r.ContentLength)
	_, err := r.Body.Read(body)
	err = json.Unmarshal(body, clt)
	c.Infof("addclient: got %v\n", string(body))
	if err != nil {
		c.Errorf("unmarshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = checkClientRequired(clt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if clt.Ethnicity == "" {
		clt.Ethnicity = "UNK"
	}

	if err = checkClientValues(clt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ikey := datastore.NewIncompleteKey(c, "SVDPClient", nil)
	key, err := datastore.Put(c, ikey, clt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	created := new(update)
	created.User = user.Current(c).String()
	created.When = time.Now().String()
	ikey = datastore.NewIncompleteKey(c, "SVDPUpdate", key)
	_, err = datastore.Put(c, ikey, created)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newrec := &clientrec{key.IntID(),
		client{clt.Firstname, clt.Lastname, clt.Address, clt.Apt,
			clt.CrossStreet, clt.DOB, clt.Phonenum, clt.Altphonenum,
			clt.Altphonedesc, clt.Ethnicity, clt.ReferredBy,
			clt.Notes, clt.Adultmales, clt.Adultfemales,
			clt.Fammbrs, clt.Financials},
	}
	b, err := json.Marshal(newrec)
	c.Infof("returning %v\n", string(b))
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, string(b))
}

func editclient(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	clt := &client{}
	body := make([]byte, r.ContentLength)
	_, err := r.Body.Read(body)
	err = json.Unmarshal(body, clt)
	c.Infof("api/editclient: got %v\n", string(body))
	if err != nil {
		c.Errorf("unmarshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = checkClientRequired(clt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if clt.Ethnicity == "" {
		clt.Ethnicity = "UNK"
	}

	if err = checkClientValues(clt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	re, err := regexp.Compile("[0-9]+")
	idstr := re.FindString(r.URL.Path)
	c.Debugf("parsed id %v from %v", idstr, r.URL.Path)

	if idstr == "" {
		c.Errorf("id is missing for update request: path %v, data %v",
			r.URL.Path, string(body))
		http.Error(w,
			fmt.Sprintf("id is missing in path for update request %v", string(body)),
			http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		c.Errorf("unable to parse id %v as int64: %v", id, err.Error())
		http.Error(w,
			fmt.Sprintf("unable to parse id %v as int64: %v", id,
				err.Error()),
			http.StatusBadRequest)
		return
	}

	if len(clt.DOB) > 0 {
		if _, err = time.Parse("2006-01-02", clt.DOB); err != nil {
			c.Errorf("unable to parse DOB %v, err %v",
				clt.DOB, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	ikey := datastore.NewKey(c, "SVDPClient", "", id, nil)
	key, err := datastore.Put(c, ikey, clt)
	if err != nil {
		c.Errorf("datastore error on Put: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	latest := new(update)
	latest.User = user.Current(c).String()
	latest.When = time.Now().String()
	ikey = datastore.NewIncompleteKey(c, "SVDPUpdate", key)
	_, err = datastore.Put(c, ikey, latest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newrec := &clientrec{key.IntID(), *clt}

	b, err := json.Marshal(newrec)
	if err != nil {
		c.Errorf("marshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Infof("returning %v\n", string(b))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(b))
}

func checkClientRequired(clt *client) error {

	onlyWS, err := regexp.Compile(`^[\s]*$`)
	if err != nil {
		return err
	}
	if onlyWS.MatchString(clt.Firstname) {
		return errors.New("Firstname is empty and cannot be")
	}
	if onlyWS.MatchString(clt.Lastname) {
		return errors.New("Lastname is empty and cannot be")
	}
	return nil
}

func checkClientValues(clt *client) error {

	if ethnicities[clt.Ethnicity] {
		return nil
	}
	var valid []byte
	for k, _ := range ethnicities {
		valid = append(valid, (k + ",")...)
	}
	return errors.New("Ethnicity must be one of " + string(valid))
}

func checkVisitRequired(vst *visit) error {

	onlyWS, err := regexp.Compile(`^[\s]*$`)
	if err != nil {
		return err
	}
	if onlyWS.MatchString(vst.Visitdate) {
		return errors.New("Visitdate is empty and cannot be")
	}
	if onlyWS.MatchString(vst.Vincentians) {
		return errors.New("Vincentians is empty and cannot be")
	}
	return nil
}

func visitrouter(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	/*
	 /api/visit/123 goes to addvisit,
	 /api/visit/123/456/edit goes to editvisit
	*/
	if !apiuserOK(c, w) {
		return
	}

	re, err := regexp.Compile(`^/api/visit/([0-9]+)(/[0-9]+/edit)?$`)
	if err != nil {
		c.Debugf("failed to create expr: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	matches := re.FindSubmatch([]byte(r.URL.Path))
	if matches != nil {
		c.Debugf("found %v matches in %v", len(matches), r.URL.Path)
		for i, s := range matches {
			c.Debugf("%v: %v", i, string(s))
		}
	} else {
		c.Debugf("no matches in %v", r.URL.Path)
		http.Error(w, "no matches in url path", http.StatusBadRequest)
		return
	}
	if len(matches[2]) > 0 {
		editvisit(c, w, r)
	} else {
		addvisit(c, w, r)
	}
}

func addvisit(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	vst := &visit{}
	body := make([]byte, r.ContentLength)
	_, err := r.Body.Read(body)
	err = json.Unmarshal(body, vst)
	c.Infof("api/addvisit: got %v\n", string(body))
	if err != nil {
		c.Errorf("unmarshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = checkVisitRequired(vst); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	re, err := regexp.Compile("[0-9]+")
	idstr := re.FindString(r.URL.Path)
	c.Debugf("parsed id %v from %v", idstr, r.URL.Path)

	if idstr == "" {
		c.Errorf("id is missing for add visit request: path %v, data %v",
			r.URL.Path, string(body))
		http.Error(w,
			fmt.Sprintf("id is missing in path for add visit request %v", string(body)),
			http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		c.Errorf("unable to parse id %v as int64: %v", id, err.Error())
		http.Error(w,
			fmt.Sprintf("unable to parse id %v as int64: %v", id,
				err.Error()),
			http.StatusBadRequest)
		return
	}

	if len(vst.Visitdate) > 0 {
		if _, err = time.Parse("2006-01-02", vst.Visitdate); err != nil {
			c.Errorf("unable to parse visit date %v, err %v",
				vst.Visitdate, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	ikey := datastore.NewIncompleteKey(c, "SVDPClientVisit",
		datastore.NewKey(c, "SVDPClient", "", id, nil))
	key, err := datastore.Put(c, ikey, vst)
	if err != nil {
		c.Errorf("datastore error on Put: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	created := new(update)
	created.User = user.Current(c).String()
	created.When = time.Now().String()

	ikey = datastore.NewIncompleteKey(c, "SVDPUpdate", key)
	_, err = datastore.Put(c, ikey, created)
	if err != nil {
		c.Errorf("datastore error on Put: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newrec := &visitrec{key.IntID(), id, *vst}

	b, err := json.Marshal(newrec)
	if err != nil {
		c.Errorf("marshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Infof("returning %v\n", string(b))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(b))
}

func editvisit(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	vst := &visit{}
	body := make([]byte, r.ContentLength)
	_, err := r.Body.Read(body)
	err = json.Unmarshal(body, vst)
	c.Infof("api/editvisit: got %v\n", string(body))
	if err != nil {
		c.Errorf("unmarshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = checkVisitRequired(vst); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	re, err := regexp.Compile(`^/api/visit/([0-9]+)/([0-9]+)/edit$`)
	matches := re.FindSubmatch([]byte(r.URL.Path))
	if matches == nil {
		http.Error(w,
			fmt.Sprintf("id is missing in path for update request %v", string(body)),
			http.StatusBadRequest)
		return
	}
	cltidstr := string(matches[1])
	vstidstr := string(matches[2])

	c.Debugf("parsed id clt %v, vst %v from %v", cltidstr, vstidstr, r.URL.Path)

	if cltidstr == "" {
		c.Errorf("cltid is missing for update request: path %v, data %v",
			r.URL.Path, string(body))
		http.Error(w,
			fmt.Sprintf("id is missing in path for update request %v", string(body)),
			http.StatusBadRequest)
		return
	}

	if vstidstr == "" {
		c.Errorf("vstid is missing for update request: path %v, data %v",
			r.URL.Path, string(body))
		http.Error(w,
			fmt.Sprintf("id is missing in path for update request %v", string(body)),
			http.StatusBadRequest)
		return
	}

	cltid, err := strconv.ParseInt(cltidstr, 10, 64)
	if err != nil {
		c.Errorf("unable to parse id %v as int64: %v", cltid, err.Error())
		http.Error(w,
			fmt.Sprintf("unable to parse id %v as int64: %v", cltid,
				err.Error()),
			http.StatusBadRequest)
		return
	}

	vstid, err := strconv.ParseInt(vstidstr, 10, 64)
	if err != nil {
		c.Errorf("unable to parse vst id %v as int64: %v", vstid, err.Error())
		http.Error(w,
			fmt.Sprintf("unable to parse id %v as int64: %v", vstid,
				err.Error()),
			http.StatusBadRequest)
		return
	}

	ikey := datastore.NewKey(c, "SVDPClientVisit", "", vstid,
		datastore.NewKey(c, "SVDPClient", "", cltid, nil))
	key, err := datastore.Put(c, ikey, vst)
	if err != nil {
		c.Errorf("datastore error on Put: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	latest := new(update)
	latest.User = user.Current(c).String()
	latest.When = time.Now().String()

	ikey = datastore.NewIncompleteKey(c, "SVDPUpdate", key)
	_, err = datastore.Put(c, ikey, latest)
	if err != nil {
		c.Errorf("datastore error on Put: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newrec := &visitrec{key.IntID(), key.Parent().IntID(), *vst}

	b, err := json.Marshal(newrec)
	if err != nil {
		c.Errorf("marshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Infof("returning %v\n", string(b))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(b))
}

func getallclients(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	q := datastore.NewQuery("SVDPClient")
	var clients []client
	ids, err := q.GetAll(c, &clients)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Debugf("getallclients: got keys %v\n", ids)
	w.WriteHeader(http.StatusOK)

	clientrecs := make([]clientrec, len(clients))
	for i := 0; i < len(clients); i++ {
		clientrecs[i] = clientrec{ids[i].IntID(), client{clients[i].Firstname,
			clients[i].Lastname, clients[i].Address,
			clients[i].Apt, clients[i].CrossStreet, clients[i].DOB,
			clients[i].Phonenum, clients[i].Altphonenum,
			clients[i].Altphonedesc, clients[i].Ethnicity,
			clients[i].ReferredBy, clients[i].Notes,
			clients[i].Adultmales, clients[i].Adultfemales,
			clients[i].Fammbrs, clients[i].Financials}}
	}
	c.Debugf("getallclients: clientrecs = %v\n", clientrecs)
	b, err := json.Marshal(clientrecs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(b))
}

func getallvisits(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	re, err := regexp.Compile("[0-9]+")
	idstr := re.FindString(r.URL.Path)
	c.Debugf("parsed id %v from %v", idstr, r.URL.Path)

	if idstr == "" {
		c.Errorf("id is missing for request: path %v", r.URL.Path)
		http.Error(w,
			fmt.Sprintf("id is missing in path %v", r.URL.Path),
			http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		c.Errorf("unable to parse id %v as int64: %v", id, err.Error())
		http.Error(w,
			fmt.Sprintf("unable to parse id %v as int64: %v", id,
				err.Error()),
			http.StatusBadRequest)
		return
	}
	q := datastore.NewQuery("SVDPClientVisit").Ancestor(datastore.NewKey(
		c, "SVDPClient", "", id, nil))
	var visits []visit
	ids, err := q.GetAll(c, &visits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Debugf("getallvisits: got keys %v\n", ids)
	w.WriteHeader(http.StatusOK)

	visitrecs := make([]visitrec, len(visits))
	for i := 0; i < len(visits); i++ {
		visitrecs[i] = visitrec{ids[i].IntID(), id,
			visit{visits[i].Vincentians, visits[i].Visitdate,
				visits[i].Assistancerequested, visits[i].Giftcardamt,
				visits[i].Numfoodboxes, visits[i].Rentassistance,
				visits[i].Utilitiesassistance,
				visits[i].Waterbillassistance,
				visits[i].Otherassistancetype,
				visits[i].Otherassistanceamt,
				visits[i].Vouchersclothing, visits[i].Vouchersfurniture,
				visits[i].Vouchersother, visits[i].Comment}}
	}
	c.Debugf("getallclients: visitrecs = %v\n", visitrecs)
	b, err := json.Marshal(visitrecs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(b))
}

func getvisitsinrange(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	q := datastore.NewQuery("SVDPClientVisit").Order("-Visitdate")
	var visits []visit
	ids, err := q.GetAll(c, &visits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Debugf("getallvisits: got keys %v\n", ids)
	w.WriteHeader(http.StatusOK)

	b, err := json.Marshal(visits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(b))
}

func editusers(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	u := user.Current(c)
	admin, err := useradmin(c, u.Email)
	if !admin {
		c.Errorf("user %v is not admin", u.Email)
		http.Error(w, "Sorry, you must be an admin user and you're not",
			http.StatusForbidden)
		return
	}

	var b1 useredit

	body := make([]byte, r.ContentLength)
	_, err = r.Body.Read(body)
	err = json.Unmarshal(body, &b1)
	c.Infof("api/editusers: got %v\n", string(body))
	c.Infof("api/editusers: unmarshaled into %v\n", b1)
	if err != nil {
		c.Errorf("unmarshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(b1.Ids) < len(b1.Aus) {
		c.Errorf("%v ids but %v users", len(b1.Ids), len(b1.Aus))
		http.Error(w,
			fmt.Sprintf("Must have as many Ids as Aus (sent %v  Ids but %v Aus)", len(b1.Ids), len(b1.Aus)),
			http.StatusBadRequest)
		return
	}

	keys := make([]*datastore.Key, len(b1.Aus))
	for i := 0; i < len(b1.Aus); i++ {
		keys[i] = datastore.NewKey(c, "SVDPUser", "", b1.Ids[i],
			nil)
	}
	newkeys, err := datastore.PutMulti(c, keys, b1.Aus)
	if err != nil {
		c.Errorf("datastore error on PutMulti: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i := 0; i < len(newkeys); i++ {
		b1.Ids[i] = newkeys[i].IntID()
	}

	if len(b1.DeletedIds) > 0 {
		deletedkeys := make([]*datastore.Key, len(b1.DeletedIds))
		for i := 0; i < len(b1.DeletedIds); i++ {
			deletedkeys[i] = datastore.NewKey(c, "SVDPUser", "",
				b1.DeletedIds[i], nil)
		}
		if err = datastore.DeleteMulti(c, deletedkeys); err != nil {
			c.Errorf("error deleting users: %v", err)
		}
	}

	latest := new(update)
	latest.User = u.String()
	latest.When = time.Now().String()

	ikey := datastore.NewIncompleteKey(c, "SVDPUserUpdate", nil)
	_, err = datastore.Put(c, ikey, latest)
	if err != nil {
		c.Errorf("datastore error: :%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nb, err := json.Marshal(&b1)
	if err != nil {
		c.Errorf("marshaling error:%v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Infof("returning %v\n", string(nb))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(nb))
}

func getallusers(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !apiuserOK(c, w) {
		return
	}

	q := datastore.NewQuery("SVDPUser").Order("Email")
	var aus []appuser

	keys, err := q.GetAll(c, &aus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Debugf("getallusers: got keys %v\n", keys)

	var resp useredit
	resp.Aus = aus
	resp.Ids = make([]int64, len(keys))
	for i := 0; i < len(keys); i++ {
		resp.Ids[i] = keys[i].IntID()
	}

	c.Debugf("getallusers: useredit = %v", resp)
	b, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(b))
}
