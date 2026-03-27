/**
 * NPS 通用JavaScript函数
 */

/**
 * 初始化数据表格
 * @param {string} selector 表格选择器
 * @param {object} options 表格配置选项
 * @returns {object} 表格对象
 */
function initTable(selector, options) {
    var defaultOptions = {
        method: 'post',
        contentType: 'application/x-www-form-urlencoded',
        dataType: 'json',
        striped: true,
        pagination: true,
        pageSize: 10,
        pageList: [10, 25, 50, 100],
        search: false,
        showColumns: false,
        showRefresh: false,
        minimumCountColumns: 2,
        clickToSelect: true,
        showToggle: false,
        cardView: false,
        detailView: false,
        locale: 'zh-CN'
    };

    // 合并默认选项和用户选项
    var tableOptions = $.extend({}, defaultOptions, options);
    
    // 初始化表格
    $(selector).bootstrapTable(tableOptions);
    
    // 返回表格对象，提供简便的刷新和重载方法
    return {
        bootstrapTable: function(method) {
            return $(selector).bootstrapTable(method);
        },
        reload: function() {
            $(selector).bootstrapTable('refresh');
        },
        getData: function() {
            return $(selector).bootstrapTable('getData');
        }
    };
} 