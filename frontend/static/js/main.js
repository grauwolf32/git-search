HighlightedReport = Vue.component('h-report', {
    props: ["fragment"],
    render(new_el) {
        var text = this.fragment.text
        var ind  = this.fragment.ids
        var rootChilds = []

        rootChilds.push(new_el("span", {}, text.substring(0, ind[0])))
        for(var i=0; i < ind.length; i += 2)
        {
            rootChilds.push(new_el("span", {class : "highlight"}, text.substring(ind[i], ind[i+1])))
            if(i+2 < ind.length){
                rootChilds.push(new_el("span", {},  text.substring(ind[i+1], ind[i+2])))
            }
        }

        rootChilds.push(new_el("span", {},  text.substring(ind[ind.length - 1], text.length)))
        return divElement = new_el("div", {class:"text-wrap"}, rootChilds) 
    },
  })

HeadNavigation = Vue.component('h-nav', {
      data: function(){
          return {
            navigation: {
                name : "Megascan",
                pages : [{
                    name: "Github",
                    path: "/github"
                },
                { 
                    name:"Gist",
                    path:"/gist"
                },
                {
                    name:"Settings",
                    path:"/settings"
                }]
            }
          }
      },
      template:  `<nav class="navbar navbar-expand-md navbar-dark fixed-top bg-dark">
                 <a class="navbar-brand" href="#">{{navigation.name}}</a>
                 <div class="collapse navbar-collapse" id="navbarCollapse">
                 <ul class="navbar-nav mr-auto">
                      <li v-for="page in navigation.pages" v-bind:class="[page.path == $route.path ? 'nav-item active': 'nav-item']">
                        <router-link class="nav-link" v-bind:to="page.path">{{page.name}}</router-link>
                      </li>
                 </ul></div></nav>`
})

Pagination = Vue.component('p-nav', {
    props: ["pagination"],
    template: `<nav aria-label="Page navigation example">
                <ul class="pagination justify-content-center">
                    <li class="page-item"><a class="page-link" v-on:click="$emit('goTo', 0)">&lt;&lt;</a></li>
                    <li class="page-item"><a class="page-link" v-on:click="$emit('skipLeft')">&lt;</a></li>
                    <li v-for="page in pagination.pages"  v-bind:class="[page.id == pagination.currentPage ? 'page-item active' : 'page-item']">
                        <a  v-on:click="$emit('goTo', page.id)" class="page-link">
                            {{page.id}}
                        </a>
                    </li> 
                    <li class="page-item"><a class="page-link" v-on:click="$emit('skipRight')">&gt;</a></li>
                    <li class="page-item"><a class="page-link" v-on:click="$emit('goTo', pagination.maxPage)">&gt;&gt;</a></li>
                </ul></nav>`
})

RControl = Vue.component('r-control',{
    props:["fragment"],
    data : function(){
        return{
            buttons : [
                {name: "Verify", action: 2},
                {name: "Close",  action: 1},
                {name: "Info",   action: 0}
            ]
        }
    },
    template: `<div>
                    <button v-for="button in buttons" v-on:click="$emit('markResult', [button.action, fragment.id])"  type="button" class="btn btn-outline-primary">{{button.name}}</button>
               </div>`
})

VItems = Vue.component('v-items',{
    props:["vitem"],
    template: `
            <div>
                <label class="form-label"> {{vitem.name}}</label><br>
                <div class="input-group mb-3">
                <select class="form-select select-item" multiple>
                    <option class="list-group-item" v-for="item in vitem.data"> {{ item }} </option>
                </select>
                 </div>
    
                 <div class="input-group mb-3">
                <input type="text" class="form-control input-item"></input>
                <button type="button" class="btn btn-outline-primary">Add</button>
                <button type="button" class="btn btn-outline-primary">Remove</button>
                </div>
            </div>`
})

Settings = Vue.component('settings', {
    data : function(){
        return {info: {}}
    },
    methods:{
        getInfo(){
            var requestURI = '/api/info'
            axios.get(requestURI)
                .then(response => {
                    this.info = response.data
                    console.log(this.info)
                })
                .catch(error => {
                    console.log(error);
                })
        }
    },
    created : function(){
        this.getInfo()
    },
    template: `
    <div>
        <br/><br/><br/><br/>
        
        <table class="table">
        <tbody><tr><td>
        <div class="container">
            <h3>Database Settings</h3>
            <label class="form-label"> Database:</label><br>
            <div class="input-group mb-3">
                <input type="text" class="form-control" 
                            placeholder="Database" 
                            aria-label="Database" 
                            v-model="info.db_redentials.database">
            </div>
            <label class="form-label"> Username:</label><br>
            <div class="input-group mb-3">             
                <input type="text" class="form-control" 
                placeholder="Username" 
                aria-label="Username" 
                v-model="info.db_redentials.name">
            </div>

            <label class="form-label"> Password:</label><br>
            <div class="input-group mb-3"> 
                <input type="text" class="form-control" placeholder="Username" aria-label="Password" v-model="info.db_redentials.password">
            </div>
            <button type="button" class="btn btn-outline-primary">Update</button>
        </div>
        </td>

        <td>
        <div class="container">
            <h3> Github Settings </h3>
            <v-items v-bind:vitem="{name:'Tokens', data:info.github.tokens}"></v-items>
            <v-items v-bind:vitem="{name:'Languages', data:info.github.langs}"></v-items>
        </div>
        </td></tr> <tr>
            <td colspan="2">
            <h3>Global Settings</h3>
            <v-items v-bind:vitem="{name:'Keywords', data:info.globals.keywords}"></v-items>
            </td>
        </tr>
        </tbody></table>
    </div>
    `
})


Fragments = Vue.component('r-fragments', {
    props: ["pagetype"],

    data: function() {
        return {
            fragments : [],
            pagination: {
                pages:[],
                maxPage: 0,
                currentPage : 0
            },
            reportStatuses:[
                {name: "New",    value: "new"},
                {name: "Closed", value: "closed"},
            ],
            availableLimits:[10, 20, 50, 100],
            reportStatus: "new",
            limit: 10
        } 
    },
    
    template: `<div><br/><br/>
                <table class="table table-bordered fixed">
                <thead>
                <tr> <th>
                    Report status: 
                    <select v-model="reportStatus" v-on:change="updatePage()">
                        <option v-for="status in reportStatuses" 
                                v-bind:value="status.value" 
                                v-bind:selected="[status.value == reportStatus]">
                                {{status.name}}
                        </option>
                    </select>

                    Limit:
                    <select v-model="limit" v-on:change="updatePage()">
                        <option v-for="lim in availableLimits" 
                                v-bind:value="lim"
                                v-bind:selected="[lim == 10]"> 
                                {{ lim }}
                        </option>
                    </select>
                </th></tr>
                </thead>
                <tbody>
                        <tr v-for="fragment in fragments">
                            <td style="white-space:pre; width: 900px; font-size: 10; word-break: break-all;"> 
                                <h-report  v-bind:fragment="fragment" v-bind:key="fragment.id"></h-report>
                                <r-control v-bind:fragment="fragment" v-on:markResult="markResult($event)"></r-control>
                            </td>
                        </tr>
                </tbody>
                </table>
                <p-nav v-bind:pagination="pagination" 
                       v-on:goTo="goTo($event)" 
                       v-on:skipLeft="skipLeft"
                       v-on:skipRight="skipRight"></p-nav>
                </div>`,
    methods: {
            updatePage: function () {
            offset = this.pagination.currentPage*this.limit
            var requestURI = '/api/get/' + this.pagetype + "/" +  this.reportStatus + '?limit=' + this.limit + '&offset=' + offset

            axios.get(requestURI)
                .then(response => {
                    this.fragments = response.data["fragments"]
                    var nResults = response.data["total_count"]
                    
                    this.updatePagination(nResults)
                })
                .catch(error => {
                    console.log(error);
                })
            },
        
        markResult: function(data){
            var fragment_id = data[1]
            var status = data[0]

            if(status == 1){
                for(var i = 0; i < this.fragments.length;i++){
                    if (this.fragments[i].id == fragment_id){
                        this.fragments.splice(i, 1)
                        break
                    }
                }
            }
        },

        updatePagination: function(nResults){
            this.pagination.maxPage =  Math.ceil(nResults / this.limit) 
            currentPage = this.pagination.currentPage
            maxPage = this.pagination.maxPage

            var st  = 0
            var end = 0

            if (maxPage - currentPage >= 5 && currentPage >= 5){
                st  = currentPage - 5
                end = currentPage + 5
            } else if(currentPage < 5){
                var nLeft = 5 - currentPage
                if (maxPage - currentPage >= 5 + nLeft){
                    end = currentPage + 5 + nLeft
                } else {
                    end = maxPage
                }
            } else {
                st = currentPage - 5
                end = maxPage
            }

            var pagination = []
            for(var i=st; i < end;i++){
                pagination.push({id: i})
            }

            this.pagination.pages = pagination
            return
        },
        goTo: function(currentPage){
            if (currentPage > this.pagination.maxPage){
                this.pagination.currentPage = this.maxPage
            }else if (currentPage < 0){
                this.pagination.currentPage = 0
            }else {
                this.pagination.currentPage = currentPage
            }
            this.updatePage()
        },

        skipRight: function(){
            this.goTo(this.pagination.currentPage+10)
        },
        skipLeft: function(){
            this.goTo(this.pagination.currentPage-10)
        }
    },
    created: function(){
        this.updatePage()
        return
    }
})


const router = new VueRouter({
    routes :[ 
        {path: "/", component:Fragments, props:{pagetype:"github"}},
        {path: "/github", component:Fragments, props:{pagetype:"github"}},
        {path: "/gist",  component:Fragments, props:{pagetype:"github"}},
        {path: "/settings",  component:Settings }
    ],
    mode: "history"
})

var app = new Vue({
    el: '#app',
    data: {
        pagetype: "github",
    },
    components:{
        'report-highlight': HighlightedReport,
        'header-navigation': HeadNavigation,
        'pagination-navigation' : Pagination,
        'report-control' : RControl,
        'fragments' : Fragments,
        'settings' : Settings,
        'v-items' : VItems,
    },
    router: router
})