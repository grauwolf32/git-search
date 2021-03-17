var app = new Vue({
    el: '#app',
    data: {
        requests : [],
        pageTypesAvailable : ["github", "gist"],
        resultPages : 0,
        pageType: "github",
        currPage : 0,
        resp : false,
        init : true
    },
    methods : {
        getResults: function (datatype, status, limit, offset) {
            axios.get('/api/get/' + datatype + "/" +  status + '?limit=' + limit + '&offset=' + offset)
                .then(response => {
                    var fragments = response.data["fragments"]
                
                    this.requests = fragments
                    this.resultPages = response.data["pages"]
                    console.log(response);
                })
                .catch(error => {
                    // handle error
                    console.log(error);
                })
        },
        markAsFalse: function(datatype, id){
            console.log(datatype)
        },
        goTo: function(pageType){
            for(var i = 0; i < this.pageTypesAvailable.length;i++){
                if(this.pageTypesAvailable[i] == pageType){
                    this.pageType = pageType
                }
            }
        }
    },
    created: function(){
        this.getResults(this.pageType, "new", 10, 0)
    }
})